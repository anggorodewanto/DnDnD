package inventory

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
)

// RechargeInfo describes how a magic item recharges at dawn.
type RechargeInfo struct {
	Dice          string // e.g. "1d6+1"
	DestroyOnZero bool   // if true and charges are 0, roll d20 — on 1, item is destroyed
}

// DawnRechargeInput holds parameters for dawn recharge processing.
type DawnRechargeInput struct {
	Items        []character.InventoryItem
	RechargeInfo map[string]RechargeInfo // keyed by item ID
}

// RechargedItem records what happened to a single item during dawn recharge.
type RechargedItem struct {
	ItemID    string
	Name      string
	Restored  int
	NewTotal  int
	Destroyed bool
}

// DawnRechargeResult holds the result of dawn recharge processing.
type DawnRechargeResult struct {
	UpdatedItems []character.InventoryItem
	Recharged    []RechargedItem
}

// DawnRecharge processes dawn recharge for all magic items with recharge info.
// It rolls recharge dice, restores charges (capped at max), and handles destroy-on-zero.
func (s *Service) DawnRecharge(input DawnRechargeInput) (DawnRechargeResult, error) {
	var result []character.InventoryItem
	var recharged []RechargedItem

	for _, item := range input.Items {
		info, ok := input.RechargeInfo[item.ItemID]
		if !ok {
			result = append(result, item)
			continue
		}

		// Roll recharge dice
		roll, err := s.roller.Roll(info.Dice)
		if err != nil {
			return DawnRechargeResult{}, fmt.Errorf("rolling recharge for %s: %w", item.Name, err)
		}

		restored := roll.Total
		newCharges := item.Charges + restored
		if newCharges > item.MaxCharges {
			restored = item.MaxCharges - item.Charges
			newCharges = item.MaxCharges
		}

		item.Charges = newCharges
		result = append(result, item)
		recharged = append(recharged, RechargedItem{
			ItemID:   item.ItemID,
			Name:     item.Name,
			Restored: restored,
			NewTotal: newCharges,
		})
	}

	return DawnRechargeResult{
		UpdatedItems: result,
		Recharged:    recharged,
	}, nil
}
