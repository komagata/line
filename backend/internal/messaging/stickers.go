package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kongesque/line-cli/internal/session"
	"github.com/kongesque/line-cli/pkg/line"
)

var stickerDecimal = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
var stickerHash = regexp.MustCompile(`^[a-fA-F0-9]{1,128}$`)
var stickerOptions = []string{"", "A", "S", "AS", "P", "PS", "T", "CT"}

func ValidStickerID(s string) bool {
	if !stickerDecimal.MatchString(s) {
		return false
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}
func stickerLabel(s string) string {
	if !utf8.ValidString(s) {
		return ""
	}
	r := []rune(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return -1
		}
		return r
	}, s))
	if len(r) > 160 {
		r = r[:160]
	}
	return string(r)
}

type Sticker struct {
	ID        string `json:"id"`
	PackageID string `json:"packageId"`
	Version   string `json:"version"`
	Option    string `json:"option"`
	Hash      string `json:"hash"`
	Alt       string `json:"alt"`
}

func projectSticker(m map[string]string) *Sticker {
	option := m["STKOPT"]
	// LINE's received static stickers encode the empty option as "0".
	// Normalize only this wire representation; outbound option validation stays unchanged.
	if option == "0" {
		option = ""
	}
	if !ValidStickerID(m["STKID"]) || (m["STKPKGID"] != "" && !ValidStickerID(m["STKPKGID"])) || (m["STKVER"] != "" && !ValidStickerID(m["STKVER"])) || (m["STKHASH"] != "" && !stickerHash.MatchString(m["STKHASH"])) {
		return nil
	}
	valid := false
	for _, v := range stickerOptions {
		if v == option {
			valid = true
		}
	}
	if !valid || option == "T" || option == "CT" {
		return nil
	}
	return &Sticker{ID: m["STKID"], PackageID: m["STKPKGID"], Version: m["STKVER"], Option: option, Hash: m["STKHASH"], Alt: stickerLabel(m["STKTXT"])}
}

type OwnedStickerRange struct {
	Start string `json:"start"`
	Size  int    `json:"size"`
}
type OwnedStickerProduct struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	ValidUntil   string              `json:"validUntil"`
	ResourceType int                 `json:"resourceType"`
	Hash         string              `json:"hash"`
	Ranges       []OwnedStickerRange `json:"ranges"`
	Count        int                 `json:"count"`
	Supported    bool                `json:"supported"`
}

func projectProduct(p line.StickerProduct, now int64) (*OwnedStickerProduct, error) {
	s := p.Summary.Sticker
	invalid := errors.New("invalid owned-sticker product")
	if !ValidStickerID(p.ID.String()) || !ValidStickerID(p.Version.String()) || s == nil || s.ResourceType < 1 || s.ResourceType > 8 || (s.Hash != "" && !stickerHash.MatchString(s.Hash)) || len(s.Ranges) == 0 || len(s.Ranges) > 64 {
		return nil, invalid
	}
	expiry, err := p.ValidUntil.Int64()
	if err != nil || expiry < -1 {
		return nil, invalid
	}
	if expiry != -1 && expiry <= now {
		return nil, nil
	}
	result := &OwnedStickerProduct{ID: p.ID.String(), Name: stickerLabel(p.Name), Version: p.Version.String(), ValidUntil: p.ValidUntil.String(), ResourceType: s.ResourceType, Hash: s.Hash, Supported: s.ResourceType <= 6, Ranges: []OwnedStickerRange{}}
	if result.Name == "" {
		result.Name = "スタンプ"
	}
	for _, r := range s.Ranges {
		if !ValidStickerID(r.Start.String()) {
			return nil, invalid
		}
		start, _ := strconv.ParseUint(r.Start.String(), 10, 64)
		size, err := r.Size.Int64()
		if err != nil || size < 1 || size > 1000 || start > math.MaxUint64-uint64(size-1) {
			return nil, invalid
		}
		result.Count += int(size)
		if result.Count > 1000 {
			return nil, errors.New("owned sticker pack exceeds 1000 stickers")
		}
		result.Ranges = append(result.Ranges, OwnedStickerRange{Start: r.Start.String(), Size: int(size)})
	}
	sort.Slice(result.Ranges, func(i, j int) bool {
		a, _ := strconv.ParseUint(result.Ranges[i].Start, 10, 64)
		b, _ := strconv.ParseUint(result.Ranges[j].Start, 10, 64)
		return a < b
	})
	var end uint64
	for _, r := range result.Ranges {
		start, _ := strconv.ParseUint(r.Start, 10, 64)
		if start <= end {
			return nil, invalid
		}
		end = start + uint64(r.Size-1)
	}
	return result, nil
}

// Optional interface keeps the existing session API and fake clients unchanged.
type stickerShop interface {
	OwnedStickerProducts(int, string) (*line.StickerProductPage, error)
}

func (c *Client) OwnedStickers() ([]OwnedStickerProduct, error) {
	var profile *line.Profile
	if err := c.Session.Do(func(api session.API) (err error) { profile, err = api.GetProfile(); return }); err != nil {
		return nil, err
	}
	if profile == nil || !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(profile.RegionCode) {
		return nil, errors.New("profile country unavailable for sticker catalog")
	}
	products := []OwnedStickerProduct{}
	seen := map[string]bool{}
	received := 0
	for offset := 0; offset <= 4000; offset += 1000 {
		var page *line.StickerProductPage
		err := c.Session.Do(func(api session.API) (err error) {
			shop, ok := api.(stickerShop)
			if !ok {
				return errors.New("sticker catalog unavailable")
			}
			page, err = shop.OwnedStickerProducts(offset, profile.RegionCode)
			return
		})
		if err != nil {
			return nil, err
		}
		if page == nil || page.Total < 0 || page.Total > 500 || len(page.Products) > 500 || page.Offset != offset {
			return nil, errors.New("owned-sticker catalog exceeds limit or has invalid pagination")
		}
		received += len(page.Products)
		if received > 500 || received > page.Total {
			return nil, errors.New("owned-sticker catalog exceeds 500 products")
		}
		for _, raw := range page.Products {
			if seen[raw.ID.String()] {
				return nil, errors.New("duplicate owned-sticker product")
			}
			seen[raw.ID.String()] = true
			p, err := projectProduct(raw, c.Session.Now().UnixMilli())
			if err != nil {
				return nil, err
			}
			if p != nil {
				products = append(products, *p)
			}
		}
		if received == page.Total {
			return products, nil
		}
		if len(page.Products) == 0 {
			return nil, errors.New("incomplete owned-sticker catalog")
		}
	}
	return nil, errors.New("owned-sticker pagination limit")
}

// Chrome's Letter Sealing content-type allowlist intentionally excludes type 7.
// This dedicated send never changes the text/file encryption or retry policies.
func (c *Client) SendSticker(chat, packageID, stickerID, replyTo string) (*SendResult, error) {
	if err := ValidateChatID(chat); err != nil {
		return nil, err
	}
	if !ValidStickerID(packageID) || !ValidStickerID(stickerID) {
		return nil, errors.New("invalid sticker selection")
	}
	if replyTo != "" {
		if err := ValidateMessageID(replyTo); err != nil {
			return nil, err
		}
	}
	products, err := c.OwnedStickers()
	if err != nil {
		return nil, err
	}
	var selected *OwnedStickerProduct
	id, _ := strconv.ParseUint(stickerID, 10, 64)
	for i := range products {
		p := &products[i]
		if p.ID != packageID || !p.Supported {
			continue
		}
		for _, r := range p.Ranges {
			start, _ := strconv.ParseUint(r.Start, 10, 64)
			if id >= start && id-start < uint64(r.Size) {
				selected = p
			}
		}
	}
	if selected == nil {
		return nil, errors.New("sticker is not owned, has expired, or requires unsupported custom text")
	}
	if toType(chat) == 0 {
		var blocked []string
		if err := c.Session.Do(func(api session.API) (err error) { blocked, err = api.GetBlockedContactIds(); return }); err != nil {
			return nil, err
		}
		for _, mid := range blocked {
			if mid == chat {
				return nil, errors.New("contact is blocked on LINE")
			}
		}
	}
	expired := func() bool {
		expiry, _ := strconv.ParseInt(selected.ValidUntil, 10, 64)
		return expiry != -1 && expiry <= c.Session.Now().UnixMilli()
	}
	if expired() {
		return nil, errors.New("selected sticker expired before sending")
	}
	seq, err := c.Session.ReserveSequence()
	if err != nil {
		return nil, err
	}
	now := c.Session.Now().UnixMilli()
	meta := map[string]string{"STKPKGID": packageID, "STKID": stickerID, "STKVER": selected.Version, "STKTXT": selected.Name}
	if opt := stickerOptions[selected.ResourceType-1]; opt != "" {
		meta["STKOPT"] = opt
	}
	if selected.Hash != "" {
		meta["STKHASH"] = selected.Hash
	}
	msg := &line.Message{ID: fmt.Sprintf("local-%d", now), From: c.state.MID, To: chat, ToType: toType(chat), CreatedTime: json.Number(strconv.FormatInt(now, 10)), ContentType: 7, HasContent: true, ContentMetadata: meta}
	if replyTo != "" {
		msg.RelatedMessageID = replyTo
		msg.MessageRelationType = 3
		msg.RelatedMessageServiceCode = 1
	}
	var sent *line.Message
	err = c.Session.Mutate(func(api session.API) (err error) {
		if expired() {
			return errors.New("selected sticker expired before sending")
		}
		sent, err = api.SendMessage(seq, msg)
		return
	})
	if err != nil {
		return nil, fmt.Errorf("sticker send did not return success; delivery may have occurred; inspect LINE before retrying: %w", err)
	}
	if sent == nil || sent.ID == "" {
		return nil, errors.New("sticker send returned no ID; delivery may have occurred; inspect LINE before retrying")
	}
	return &SendResult{ID: sent.ID, ChatID: chat, Encrypted: false, RequestSequence: seq}, nil
}
