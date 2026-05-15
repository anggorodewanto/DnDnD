finding_id: B-C01
verdict: approved
spec_alignment: confirmed
test_first_discipline: yes
unrelated_changes: none
coverage_ok: yes
new_concerns: []
notes: Replaced the broken modifier parsing (strip all + then Atoi) with a sumSignedTokens helper that regex-tokenizes signed integers and sums them. Full-coverage validation ensures no garbage characters slip through. All 4 acceptance criterion cases pass. Clean early-return style.
