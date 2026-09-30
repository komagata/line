package line

import (
	"encoding/json"
	"errors"
	"regexp"
)

// Chrome extension 3.7.2: common/fetchAllOwnedProductList -> rj.
// The old GetOwnedProductSummaries(mid) has a different, unused argument shape.
type StickerRange struct {
	Start json.Number `json:"start"`
	Size  json.Number `json:"size"`
}
type StickerSummary struct {
	ResourceType int            `json:"stickerResourceType"`
	Ranges       []StickerRange `json:"stickerIdRanges"`
	Hash         string         `json:"stickerHash"`
}
type StickerProduct struct {
	ID         json.Number `json:"id"`
	Name       string      `json:"name"`
	Version    json.Number `json:"latestVersion"`
	ValidUntil json.Number `json:"validUntil"`
	Summary    struct {
		Sticker *StickerSummary `json:"stickerSummary"`
	} `json:"productTypeSummary"`
}
type StickerProductPage struct {
	Products []StickerProduct
	Offset   int
	Total    int
}

var stickerCountry = regexp.MustCompile(`^[A-Z]{2}$`)

func (c *Client) OwnedStickerProducts(offset int, country string) (*StickerProductPage, error) {
	if offset < 0 || offset > 4000 || offset%1000 != 0 || !stickerCountry.MatchString(country) {
		return nil, errors.New("invalid owned-sticker request")
	}
	raw, err := c.callShopRPC("ShopService", "getOwnedProductSummaries", "stickershop", offset, 1000, map[string]string{"language": "ja", "country": country})
	if err != nil {
		return nil, err
	}
	var response struct {
		Code *int `json:"code"`
		Data *struct {
			Products *[]StickerProduct `json:"productList"`
			Offset   *int              `json:"offset"`
			Total    *int              `json:"totalSize"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Code == nil || *response.Code != 0 || response.Data == nil || response.Data.Products == nil || response.Data.Offset == nil || response.Data.Total == nil {
		return nil, errors.New("invalid owned-sticker response")
	}
	d := response.Data
	if *d.Offset != offset || *d.Total < 0 || *d.Total > 500 || len(*d.Products) > 500 {
		return nil, errors.New("owned-sticker catalog exceeds 500 products or has invalid pagination")
	}
	return &StickerProductPage{Products: *d.Products, Offset: *d.Offset, Total: *d.Total}, nil
}
