package shops

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// Discord caps a message at 5 action rows of 5 buttons each.
const (
	maxButtonsPerRow = 5
	maxButtonRows    = 5
	maxShopButtons   = maxButtonsPerRow * maxButtonRows
)

// BuildShopButtons builds the "Buy" action rows for a shop post. Only in-stock
// items (Quantity > 0) get a button. Buttons are chunked into action rows of 5
// and capped at 25 total (5 rows); any items beyond that remain in the post
// text but get no button. The custom ID is "shop_buy:<shopID>:<shopItemID>",
// matching the component router branch.
func BuildShopButtons(shopID uuid.UUID, items []refdata.ShopItem) []discordgo.MessageComponent {
	var buttons []discordgo.Button
	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}
		if len(buttons) >= maxShopButtons {
			break
		}
		buttons = append(buttons, discordgo.Button{
			Label:    fmt.Sprintf("Buy %s (%d gp)", item.Name, item.PriceGp),
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("shop_buy:%s:%s", shopID, item.ID),
		})
	}

	if len(buttons) == 0 {
		return nil
	}

	var rows []discordgo.MessageComponent
	for i := 0; i < len(buttons); i += maxButtonsPerRow {
		end := min(i+maxButtonsPerRow, len(buttons))
		row := make([]discordgo.MessageComponent, 0, end-i)
		for _, b := range buttons[i:end] {
			row = append(row, b)
		}
		rows = append(rows, discordgo.ActionsRow{Components: row})
	}
	return rows
}
