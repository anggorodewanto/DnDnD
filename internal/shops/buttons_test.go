package shops_test

import (
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/shops"
)

func TestBuildShopButtons_SkipsOutOfStock(t *testing.T) {
	shopID := uuid.New()
	in := refdata.ShopItem{ID: uuid.New(), Name: "Sword", PriceGp: 10, Quantity: 2}
	out := refdata.ShopItem{ID: uuid.New(), Name: "Shield", PriceGp: 5, Quantity: 0}

	comps := shops.BuildShopButtons(shopID, []refdata.ShopItem{in, out})
	require.Len(t, comps, 1)
	row := comps[0].(discordgo.ActionsRow)
	require.Len(t, row.Components, 1)
	btn := row.Components[0].(discordgo.Button)
	assert.Equal(t, fmt.Sprintf("shop_buy:%s:%s", shopID, in.ID), btn.CustomID)
	assert.Equal(t, "Buy Sword (10 gp)", btn.Label)
}

func TestBuildShopButtons_Empty(t *testing.T) {
	comps := shops.BuildShopButtons(uuid.New(), nil)
	assert.Empty(t, comps)
}

func TestBuildShopButtons_ChunksFivePerRow(t *testing.T) {
	shopID := uuid.New()
	var items []refdata.ShopItem
	for i := 0; i < 7; i++ {
		items = append(items, refdata.ShopItem{ID: uuid.New(), Name: fmt.Sprintf("I%d", i), PriceGp: 1, Quantity: 1})
	}
	comps := shops.BuildShopButtons(shopID, items)
	require.Len(t, comps, 2) // 5 + 2
	assert.Len(t, comps[0].(discordgo.ActionsRow).Components, 5)
	assert.Len(t, comps[1].(discordgo.ActionsRow).Components, 2)
}

func TestBuildShopButtons_CapsAt25(t *testing.T) {
	shopID := uuid.New()
	var items []refdata.ShopItem
	for i := 0; i < 30; i++ {
		items = append(items, refdata.ShopItem{ID: uuid.New(), Name: fmt.Sprintf("I%d", i), PriceGp: 1, Quantity: 1})
	}
	comps := shops.BuildShopButtons(shopID, items)
	require.Len(t, comps, 5) // max 5 rows
	total := 0
	for _, c := range comps {
		total += len(c.(discordgo.ActionsRow).Components)
	}
	assert.Equal(t, 25, total) // capped at 25 buttons
}
