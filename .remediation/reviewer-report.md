finding_id: A-C02
verdict: approved
spec_alignment: confirmed
test_first_discipline: yes
unrelated_changes: none
coverage_ok: yes
new_concerns: []
notes: Added checkCampaignOwnership helper that resolves the DM's campaign and compares against the approval's CampaignID. Applied to Approve and parseFeedbackRequest (shared by Reject/RequestChanges). CampaignID field added to ApprovalEntry/ApprovalDetail and populated from the store. Cross-campaign test covers all three endpoints. Diff is tight and follows early-return style.
