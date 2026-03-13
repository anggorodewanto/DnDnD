#!/usr/bin/env bash
set -euo pipefail

# ===========================================================================
# auto-implement.sh — Automated phase implementation loop for Claude Code
#
# Runs /implement-phase in a loop via claude -p, clearing context between
# phases. Detects questions (drops into interactive mode), rate limits
# (waits with countdown), and completion (exits).
#
# Usage:
#   ./auto-implement.sh                  # start from next unchecked phase
#   ./auto-implement.sh 12               # start from phase 12
#   COOLDOWN=3600 ./auto-implement.sh    # 1 hour cooldown on rate limit
# ===========================================================================

# === CONFIGURATION (override via environment) ===
PHASES_FILE="${PHASES_FILE:-docs/phases.md}"
SKILL_FILE="${SKILL_FILE:-.claude/skills/implement-phase/SKILL.md}"
COOLDOWN="${COOLDOWN:-1800}"                # seconds to wait on rate limit (default: 30 min)
MAX_TURNS="${MAX_TURNS:-200}"               # max agentic turns per run
MODEL="${MODEL:-}"                          # optional model override (e.g., "opus")
FALLBACK_MODEL="${FALLBACK_MODEL:-sonnet}"  # fallback when primary model is overloaded
START_PHASE="${1:-}"                        # optional: start from specific phase

QUESTION_MARKER="USER_INPUT_REQUIRED"

# All tools EXCEPT AskUserQuestion — forces the agent to stop and output
# questions as text (which the script catches), rather than hanging.
ALLOWED_TOOLS="Agent,Read,Edit,Write,Glob,Grep,Bash,Skill,TaskCreate,TaskGet,TaskList,TaskOutput,TaskUpdate,WebFetch,WebSearch,LSP,NotebookEdit"

# === STATE ===
PHASES_COMPLETED=0
TOTAL_COST=0
TOTAL_INPUT_TOKENS=0
TOTAL_OUTPUT_TOKENS=0
RUN_LOG="/tmp/auto-implement-runs.log"

# === COLORS ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

# === FUNCTIONS ===

get_next_phase() {
    if [ -n "$START_PHASE" ]; then
        local phase="$START_PHASE"
        START_PHASE=""  # only use once, then auto-detect
        echo "$phase"
        return
    fi
    grep -oP '\- \[ \] \*\*Phase \K\d+[a-z]?' "$PHASES_FILE" 2>/dev/null | head -1 || true
}

countdown() {
    local seconds=$1
    local msg="${2:-Resuming}"
    echo ""
    while [ "$seconds" -gt 0 ]; do
        local mins=$((seconds / 60))
        local secs=$((seconds % 60))
        printf "\r  ${CYAN}%s in ${BOLD}%02d:%02d${RESET} " "$msg" "$mins" "$secs"
        sleep 1
        seconds=$((seconds - 1))
    done
    printf "\r  ${GREEN}%s now!${RESET}                              \n" "$msg"
}

mark_phase_done() {
    local phase="$1"
    if grep -q "\[x\].*Phase ${phase}" "$PHASES_FILE" 2>/dev/null; then
        return 0  # already marked
    fi
    echo -e "  ${DIM}Marking Phase $phase done in phases.md...${RESET}"
    sed -i "s/- \[ \] \*\*Phase ${phase}:/- [x] **Phase ${phase}:/" "$PHASES_FILE" 2>/dev/null || true
    git add "$PHASES_FILE" 2>/dev/null && \
        git commit -m "Mark Phase ${phase} complete in phases.md" 2>/dev/null || true
}

is_rate_limited() {
    local text="$1"
    echo "$text" | grep -qiE \
        "rate.limit|429|overloaded|too.many.requests|resource_exhausted|capacity|usage limit"
}

log_run() {
    local phase="$1"
    local cost="$2"
    local input_tok="$3"
    local output_tok="$4"
    local duration="$5"
    local status="$6"

    # Accumulate totals
    if [ -n "$cost" ] && [ "$cost" != "null" ]; then
        TOTAL_COST=$(awk "BEGIN {printf \"%.4f\", $TOTAL_COST + $cost}")
    fi
    if [ -n "$input_tok" ] && [ "$input_tok" != "null" ]; then
        TOTAL_INPUT_TOKENS=$(awk "BEGIN {printf \"%d\", $TOTAL_INPUT_TOKENS + $input_tok}")
    fi
    if [ -n "$output_tok" ] && [ "$output_tok" != "null" ]; then
        TOTAL_OUTPUT_TOKENS=$(awk "BEGIN {printf \"%d\", $TOTAL_OUTPUT_TOKENS + $output_tok}")
    fi

    # Append to persistent log
    echo "$(date -Iseconds) phase=$phase status=$status cost=\$${cost:-0} tokens_in=${input_tok:-0} tokens_out=${output_tok:-0} duration_ms=${duration:-0} cumulative_cost=\$$TOTAL_COST" >> "$RUN_LOG"
}

print_run_stats() {
    local run_cost="$1"
    local input_tokens="$2"
    local output_tokens="$3"
    local duration_ms="$4"
    local num_turns="$5"

    local duration_str=""
    if [ -n "$duration_ms" ] && [ "$duration_ms" != "null" ]; then
        local dur_min=$((duration_ms / 60000))
        local dur_sec=$(( (duration_ms % 60000) / 1000 ))
        duration_str="${dur_min}m ${dur_sec}s"
    fi

    echo ""
    echo -e "  ${DIM}--- Run Stats ---${RESET}"
    [ -n "$run_cost" ] && [ "$run_cost" != "null" ] && \
        echo -e "  ${DIM}Run cost (API equiv):${RESET} \$${run_cost}"
    [ -n "$duration_str" ] && \
        echo -e "  ${DIM}Duration:${RESET}             ${duration_str}"
    [ -n "$num_turns" ] && [ "$num_turns" != "null" ] && \
        echo -e "  ${DIM}Turns:${RESET}                ${num_turns}"
    [ -n "$input_tokens" ] && [ "$input_tokens" != "null" ] && \
        echo -e "  ${DIM}Tokens:${RESET}               ${input_tokens} in / ${output_tokens:-0} out"
    echo -e "  ${DIM}Session total:${RESET}        \$${TOTAL_COST} across ${PHASES_COMPLETED} phase(s)"
}

print_banner() {
    echo ""
    echo -e "${BOLD}========================================${RESET}"
    echo -e "${BOLD}  Auto-Implement Loop${RESET}"
    echo -e "${BOLD}========================================${RESET}"
    echo -e "  ${DIM}Phases file:${RESET}  $PHASES_FILE"
    echo -e "  ${DIM}Cooldown:${RESET}     $((COOLDOWN / 60)) minutes"
    echo -e "  ${DIM}Max turns:${RESET}    $MAX_TURNS"
    [ -n "$MODEL" ] && echo -e "  ${DIM}Model:${RESET}        $MODEL"
    echo -e "  ${DIM}Fallback:${RESET}     $FALLBACK_MODEL"
    echo -e "  ${DIM}Run log:${RESET}      $RUN_LOG"
    echo -e "${BOLD}========================================${RESET}"
    echo ""
}

print_phase_header() {
    local phase=$1
    echo ""
    echo -e "${BLUE}----------------------------------------${RESET}"
    echo -e "${BLUE}  Phase ${BOLD}$phase${RESET}${BLUE} — Starting${RESET}"
    echo -e "${BLUE}----------------------------------------${RESET}"
    echo ""
}

print_summary() {
    echo ""
    echo -e "${BOLD}========================================${RESET}"
    echo -e "${BOLD}  Session Summary${RESET}"
    echo -e "${BOLD}========================================${RESET}"
    echo -e "  ${DIM}Phases completed:${RESET}  $PHASES_COMPLETED"
    echo -e "  ${DIM}Total cost (API):${RESET}  \$$TOTAL_COST"
    echo -e "  ${DIM}Input tokens:${RESET}      $TOTAL_INPUT_TOKENS"
    echo -e "  ${DIM}Output tokens:${RESET}     $TOTAL_OUTPUT_TOKENS"
    echo -e "  ${DIM}Run log:${RESET}           $RUN_LOG"
    echo -e "${BOLD}========================================${RESET}"
    echo ""
}

cleanup() {
    print_summary
    exit "${1:-0}"
}
trap 'cleanup 130' INT

# === PRE-FLIGHT CHECKS ===

if [ ! -f "$PHASES_FILE" ]; then
    echo -e "${RED}Error: Phases file not found: $PHASES_FILE${RESET}"
    echo "Run /breakdown-spec first to generate it."
    exit 1
fi

if [ ! -f "$SKILL_FILE" ]; then
    echo -e "${RED}Error: Skill file not found: $SKILL_FILE${RESET}"
    exit 1
fi

if ! command -v claude &>/dev/null; then
    echo -e "${RED}Error: claude CLI not found in PATH${RESET}"
    exit 1
fi

if ! command -v jq &>/dev/null; then
    echo -e "${RED}Error: jq is required. Install with: sudo apt install jq${RESET}"
    exit 1
fi

# === MAIN LOOP ===

print_banner

CONSECUTIVE_FAILURES=0
MAX_CONSECUTIVE_FAILURES=3

while true; do
    # --- Get next phase ---
    PHASE=$(get_next_phase)
    if [ -z "$PHASE" ]; then
        echo ""
        echo -e "${GREEN}${BOLD}All phases complete!${RESET}"
        cleanup 0
    fi

    print_phase_header "$PHASE"

    # --- Build claude command ---
    CMD=(claude -p "Implement Phase $PHASE of $PHASES_FILE"
        --append-system-prompt-file "$SKILL_FILE"
        --output-format json
        --max-turns "$MAX_TURNS"
        --fallback-model "$FALLBACK_MODEL"
        --dangerously-skip-permissions
    )
    [ -n "$MODEL" ] && CMD+=(--model "$MODEL")

    # --- Run implementation ---
    RESULT_FILE=$(mktemp)
    ERROR_FILE=$(mktemp)
    EXIT_CODE=0

    echo -e "  ${DIM}Running claude -p ... (this may take a while)${RESET}"
    echo -e "  ${DIM}Started at $(date '+%H:%M:%S')${RESET}"
    echo ""

    # Run claude in background so we can show elapsed time
    "${CMD[@]}" > "$RESULT_FILE" 2> "$ERROR_FILE" &
    CLAUDE_PID=$!

    # Show elapsed time while waiting (disable set -e for this section)
    ELAPSED=0
    set +e
    while kill -0 "$CLAUDE_PID" 2>/dev/null; do
        printf "\r  ${CYAN}>${RESET} ${DIM}Elapsed: %02d:%02d${RESET} " $((ELAPSED / 60)) $((ELAPSED % 60))
        sleep 1
        ELAPSED=$((ELAPSED + 1))
    done
    wait "$CLAUDE_PID"
    EXIT_CODE=$?
    set -e
    printf "\r  ${DIM}Finished after %02d:%02d${RESET}                    \n" $((ELAPSED / 60)) $((ELAPSED % 60))

    RESULT=$(cat "$RESULT_FILE")
    ERRORS=$(cat "$ERROR_FILE")
    rm -f "$RESULT_FILE" "$ERROR_FILE"

    # --- Parse JSON fields ---
    SESSION_ID=$(echo "$RESULT" | jq -r '.session_id // empty' 2>/dev/null || true)
    RESULT_TEXT=$(echo "$RESULT" | jq -r '.result // empty' 2>/dev/null || true)
    IS_ERROR=$(echo "$RESULT" | jq -r '.is_error // false' 2>/dev/null || true)
    RUN_COST=$(echo "$RESULT" | jq -r '.total_cost_usd // empty' 2>/dev/null || true)
    INPUT_TOKENS=$(echo "$RESULT" | jq -r '.usage.input_tokens // empty' 2>/dev/null || true)
    OUTPUT_TOKENS=$(echo "$RESULT" | jq -r '.usage.output_tokens // empty' 2>/dev/null || true)
    DURATION_MS=$(echo "$RESULT" | jq -r '.duration_api_ms // empty' 2>/dev/null || true)
    NUM_TURNS=$(echo "$RESULT" | jq -r '.num_turns // empty' 2>/dev/null || true)
    STOP_REASON=$(echo "$RESULT" | jq -r '.stop_reason // empty' 2>/dev/null || true)

    # --- Check for rate limit (in stderr, result text, or is_error) ---
    if is_rate_limited "$ERRORS" || is_rate_limited "$RESULT_TEXT"; then
        echo ""
        echo -e "  ${YELLOW}${BOLD}Rate limit hit!${RESET}"
        echo -e "  ${YELLOW}Claude Max limits reset on a rolling window.${RESET}"
        log_run "$PHASE" "$RUN_COST" "$INPUT_TOKENS" "$OUTPUT_TOKENS" "$DURATION_MS" "rate_limited"
        CONSECUTIVE_FAILURES=0  # rate limit isn't a failure
        countdown "$COOLDOWN" "Retrying phase $PHASE"
        START_PHASE="$PHASE"
        continue
    fi

    # --- Check is_error flag from JSON ---
    if [ "$IS_ERROR" = "true" ]; then
        CONSECUTIVE_FAILURES=$((CONSECUTIVE_FAILURES + 1))
        echo ""
        echo -e "  ${RED}Claude returned an error:${RESET}"
        echo "$RESULT_TEXT" | head -5
        log_run "$PHASE" "$RUN_COST" "$INPUT_TOKENS" "$OUTPUT_TOKENS" "$DURATION_MS" "error"

        if [ "$CONSECUTIVE_FAILURES" -ge "$MAX_CONSECUTIVE_FAILURES" ]; then
            echo ""
            echo -e "  ${RED}${BOLD}$MAX_CONSECUTIVE_FAILURES consecutive failures. Stopping.${RESET}"
            echo -e "  ${DIM}Resume with: ./auto-implement.sh $PHASE${RESET}"
            cleanup 1
        fi
        echo -e "  ${YELLOW}Attempt $CONSECUTIVE_FAILURES/$MAX_CONSECUTIVE_FAILURES${RESET}"
        countdown 60 "Retrying phase $PHASE"
        START_PHASE="$PHASE"
        continue
    fi

    # --- Check for non-zero exit without JSON error ---
    if [ "$EXIT_CODE" -ne 0 ]; then
        CONSECUTIVE_FAILURES=$((CONSECUTIVE_FAILURES + 1))
        echo ""
        echo -e "  ${RED}Error (exit code $EXIT_CODE):${RESET}"
        echo "$ERRORS" | head -5
        log_run "$PHASE" "$RUN_COST" "$INPUT_TOKENS" "$OUTPUT_TOKENS" "$DURATION_MS" "exit_error"

        if [ "$CONSECUTIVE_FAILURES" -ge "$MAX_CONSECUTIVE_FAILURES" ]; then
            echo ""
            echo -e "  ${RED}${BOLD}$MAX_CONSECUTIVE_FAILURES consecutive failures. Stopping.${RESET}"
            echo -e "  ${DIM}Resume with: ./auto-implement.sh $PHASE${RESET}"
            cleanup 1
        fi
        echo -e "  ${YELLOW}Attempt $CONSECUTIVE_FAILURES/$MAX_CONSECUTIVE_FAILURES${RESET}"
        countdown 60 "Retrying phase $PHASE"
        START_PHASE="$PHASE"
        continue
    fi

    # --- Success path ---
    CONSECUTIVE_FAILURES=0
    log_run "$PHASE" "$RUN_COST" "$INPUT_TOKENS" "$OUTPUT_TOKENS" "$DURATION_MS" "success"
    print_run_stats "$RUN_COST" "$INPUT_TOKENS" "$OUTPUT_TOKENS" "$DURATION_MS" "$NUM_TURNS"

    # --- Check for questions ---
    if echo "$RESULT_TEXT" | grep -q "$QUESTION_MARKER"; then
        echo ""
        echo -e "  ${YELLOW}${BOLD}Phase $PHASE has questions that need your input.${RESET}"
        echo ""

        # Print the questions
        echo "$RESULT_TEXT" | awk "/$QUESTION_MARKER/,0" | head -30
        echo ""

        if [ -n "$SESSION_ID" ]; then
            echo -e "  ${CYAN}Dropping into interactive mode to answer questions...${RESET}"
            echo -e "  ${DIM}Type /exit when done to continue the loop.${RESET}"
            echo ""
            claude --resume "$SESSION_ID" || true
        else
            echo -e "  ${RED}No session ID found. Answer questions manually:${RESET}"
            echo -e "  ${DIM}claude /implement-phase $PHASE${RESET}"
            cleanup 1
        fi

        # After interactive session, always mark phase done and move on.
        # The agent in --resume mode loses the system prompt context and
        # won't reliably mark the checkbox itself, so the script handles it.
        mark_phase_done "$PHASE"
        echo ""
        echo -e "  ${GREEN}Phase $PHASE completed.${RESET}"
        PHASES_COMPLETED=$((PHASES_COMPLETED + 1))
        continue
    fi

    # --- Check if phase was actually completed ---
    if grep -q "\[x\].*Phase ${PHASE}" "$PHASES_FILE" 2>/dev/null || \
       git log --oneline -5 2>/dev/null | grep -qi "Phase ${PHASE} complete"; then
        mark_phase_done "$PHASE"
        echo ""
        echo -e "  ${GREEN}${BOLD}Phase $PHASE complete!${RESET}"
        PHASES_COMPLETED=$((PHASES_COMPLETED + 1))
    else
        echo ""
        echo -e "  ${YELLOW}Phase $PHASE ran but wasn't marked complete.${RESET}"
        echo -e "  ${DIM}Stop reason: ${STOP_REASON:-unknown}${RESET}"

        if [ -n "$RESULT_TEXT" ]; then
            echo ""
            echo "$RESULT_TEXT" | tail -15
        fi

        echo ""
        echo -e "  ${CYAN}(d=mark done & continue / r=retry / q=quit)${RESET}"
        read -r choice
        case "$choice" in
            d|D) mark_phase_done "$PHASE"; PHASES_COMPLETED=$((PHASES_COMPLETED + 1)) ;;
            r|R) START_PHASE="$PHASE"; continue ;;
            q|Q) cleanup 0 ;;
            *)   mark_phase_done "$PHASE"; PHASES_COMPLETED=$((PHASES_COMPLETED + 1)) ;; # default: mark done
        esac
    fi
done
