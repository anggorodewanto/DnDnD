package itempicker

// StaticGear returns the built-in adventuring gear list (SRD).
func StaticGear() []GearItem {
	return []GearItem{
		{ID: "backpack", Name: "Backpack", Description: "A backpack can hold up to 30 pounds of gear.", CostGP: 2},
		{ID: "bedroll", Name: "Bedroll", Description: "A portable sleeping roll.", CostGP: 1},
		{ID: "crowbar", Name: "Crowbar", Description: "Grants advantage on Strength checks where leverage can be applied.", CostGP: 2},
		{ID: "grappling-hook", Name: "Grappling Hook", Description: "A metal hook attached to a rope.", CostGP: 2},
		{ID: "hammer", Name: "Hammer", Description: "A standard hammer.", CostGP: 1},
		{ID: "lantern-hooded", Name: "Lantern, Hooded", Description: "Casts bright light in a 30-foot radius.", CostGP: 5},
		{ID: "lock", Name: "Lock", Description: "A key is provided with the lock.", CostGP: 10},
		{ID: "manacles", Name: "Manacles", Description: "Metal restraints that can bind a creature.", CostGP: 2},
		{ID: "mirror-steel", Name: "Mirror, Steel", Description: "A small steel mirror.", CostGP: 5},
		{ID: "piton", Name: "Piton", Description: "An iron spike for climbing.", CostGP: 0},
		{ID: "rations-1day", Name: "Rations (1 day)", Description: "Dry food suitable for travel.", CostGP: 0},
		{ID: "rope-50ft", Name: "Rope, Hempen (50 ft)", Description: "50 feet of hempen rope. Has 2 hit points.", CostGP: 1},
		{ID: "rope-silk-50ft", Name: "Rope, Silk (50 ft)", Description: "50 feet of silk rope. Has 2 hit points.", CostGP: 10},
		{ID: "tent-two-person", Name: "Tent, Two-Person", Description: "A simple two-person tent.", CostGP: 2},
		{ID: "tinderbox", Name: "Tinderbox", Description: "Used to light a fire.", CostGP: 0},
		{ID: "torch", Name: "Torch", Description: "Burns for 1 hour, providing bright light in a 20-foot radius.", CostGP: 0},
		{ID: "waterskin", Name: "Waterskin", Description: "Holds up to 4 pints of liquid.", CostGP: 0},
	}
}

// StaticConsumables returns the built-in consumable items list (SRD).
func StaticConsumables() []ConsumableItem {
	return []ConsumableItem{
		{ID: "potion-healing", Name: "Potion of Healing", Description: "Regain 2d4+2 hit points.", CostGP: 50},
		{ID: "potion-greater-healing", Name: "Potion of Greater Healing", Description: "Regain 4d4+4 hit points.", CostGP: 150},
		{ID: "potion-superior-healing", Name: "Potion of Superior Healing", Description: "Regain 8d4+8 hit points.", CostGP: 500},
		{ID: "antitoxin", Name: "Antitoxin", Description: "Advantage on saves vs. poison for 1 hour.", CostGP: 50},
		{ID: "alchemists-fire", Name: "Alchemist's Fire", Description: "Deals 1d4 fire damage per turn on hit.", CostGP: 50},
		{ID: "holy-water", Name: "Holy Water", Description: "Deals 2d6 radiant damage to fiends and undead.", CostGP: 25},
		{ID: "oil-flask", Name: "Oil (flask)", Description: "Can be lit to deal fire damage or illuminate.", CostGP: 0},
		{ID: "acid-vial", Name: "Acid (vial)", Description: "Deals 2d6 acid damage on hit.", CostGP: 25},
	}
}
