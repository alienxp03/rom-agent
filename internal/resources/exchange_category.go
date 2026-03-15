package resources

// ExchangeCategory represents a category in the exchange/market
type ExchangeCategory struct {
	ID   int
	Name string
}

// Card categories
var (
	CardWeapon    = ExchangeCategory{1010, "Card - Weapon"}
	CardOffhand   = ExchangeCategory{1011, "Card - Off-Hand"}
	CardArmor     = ExchangeCategory{1012, "Card - Armor"}
	CardGarment   = ExchangeCategory{1013, "Card - Garment"}
	CardShoe      = ExchangeCategory{1014, "Card - Shoe"}
	CardAccessory = ExchangeCategory{1015, "Card - Accessory"}
	CardHeadwear  = ExchangeCategory{1016, "Card - Headwear"}
)

// Mount category
var Mount = ExchangeCategory{1020, "Mount"}

// Equipment categories
var (
	EqWeapon    = ExchangeCategory{1025, "Equipment - Weapon"}
	EqOffhand   = ExchangeCategory{1026, "Equipment - Off-Hand"}
	EqArmor     = ExchangeCategory{1027, "Equipment - Armor"}
	EqGarment   = ExchangeCategory{1028, "Equipment - Garment"}
	EqFootgear  = ExchangeCategory{1029, "Equipment - Footgear"}
	EqAccessory = ExchangeCategory{1030, "Equipment - Accessory"}
)

// Headwear categories
var (
	HeadHead  = ExchangeCategory{1040, "Headwear - Head"}
	HeadFace  = ExchangeCategory{1041, "Headwear - Face"}
	HeadBack  = ExchangeCategory{1042, "Headwear - Back"}
	HeadMouth = ExchangeCategory{1043, "Headwear - Mouth"}
	HeadTail  = ExchangeCategory{1044, "Headwear - Tail"}
)

// Blueprint category
var Blueprint = ExchangeCategory{12, "Blueprint"}

// Item categories
var (
	Potion   = ExchangeCategory{1001, "Item - Potion/Effect"}
	Refine   = ExchangeCategory{1002, "Item - Refine"}
	Scroll   = ExchangeCategory{1003, "Item - Scroll/Album"}
	Material = ExchangeCategory{1004, "Item - Material"}
	Holiday  = ExchangeCategory{1005, "Item - Holiday Material"}
	Pet      = ExchangeCategory{1007, "Item - Pet Material"}
)

// Costume category
var Costume = ExchangeCategory{1045, "Costume"}

// Premium category
var Premium = ExchangeCategory{1052, "Premium"}

// AllCategories contains all exchange categories in order
var AllCategories = []ExchangeCategory{
	CardWeapon, CardOffhand, CardArmor, CardGarment, CardShoe,
	CardAccessory, CardHeadwear,
	Mount,
	EqWeapon, EqOffhand, EqArmor, EqGarment, EqFootgear, EqAccessory,
	HeadHead, HeadFace, HeadBack, HeadMouth, HeadTail,
	Blueprint,
	Potion, Refine, Scroll, Material, Holiday, Pet,
	Costume,
	Premium,
}

// GetCategory returns the exchange category for the given ID
func GetCategory(id int) *ExchangeCategory {
	for i := range AllCategories {
		if AllCategories[i].ID == id {
			return &AllCategories[i]
		}
	}
	return nil
}

// GetCategoryByName returns the exchange category for the given name
func GetCategoryByName(name string) *ExchangeCategory {
	for i := range AllCategories {
		if AllCategories[i].Name == name {
			return &AllCategories[i]
		}
	}
	return nil
}
