finding_id: cross-cut-C01
severity: Critical
title: Channel Divinity recharges on long rest, not short rest
location: internal/combat/channel_divinity_integration_test.go:44
spec_ref: PHB p.59 (Cleric) / p.85 (Paladin) — "short or long rest"
problem: |
  Every test seed for the channel-divinity feature marks Recharge: "long". The rest service routes recharges by this field, so Channel Divinity only recharges on long rest instead of short rest per PHB.
suggested_fix: |
  Change the Recharge field from "long" to "short" in all channel-divinity fixtures and any character-bootstrap code. Add a regression test asserting CD is recharged after ShortRest.
acceptance_criterion: |
  Channel Divinity feature uses have Recharge: "short" in test fixtures. A test confirms ShortRest recharges channel-divinity uses.
