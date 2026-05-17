finding_id: D-H08
severity: High
title: Channel Divinity action validation duplicated and racy across DM-queue + auto-resolved paths
location: internal/combat/channel_divinity.go:160, :366, :446, :520, :590
spec_ref: spec §744-848; Phase 50
problem: |
  The DM-queue path (ChannelDivinityDMQueue) deducts the use even if the DM later rejects the effect. When s.dmNotifier == nil, a use is burned silently with no follow-up.
suggested_fix: |
  Require a notifier to be wired before allowing the deduction (return error if s.dmNotifier == nil).
acceptance_criterion: |
  ChannelDivinityDMQueue returns an error when dmNotifier is nil. A test demonstrates this.
