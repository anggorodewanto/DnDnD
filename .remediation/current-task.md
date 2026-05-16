finding_id: G-H08
severity: High
title: Long rest does not propagate dawn recharge to party rest persistence
location: internal/rest/party_handler.go:180-216
spec_ref: spec §Long Rest + §Magic Items recharge line 2707 (Phase 83b)
problem: |
  applyPartyLongRest builds LongRestInput without Inventory or RechargeInfo, so dawn-recharge never fires for party rests.
suggested_fix: |
  Extend PartyCharacterInfo to carry Inventory + RechargeInfo, pass them through.
acceptance_criterion: |
  A party long rest triggers dawn recharge for magic items. A test demonstrates charges are restored.
