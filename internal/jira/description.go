package jira

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const imageReferencePrefix = "jiratui-image:"

var imageIDRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// PlainDescription creates an editable description containing only plain text.
func PlainDescription(text string) Description {
	return Description{EditorText: normalizeNewlines(text), Editable: true}
}

// NewDescriptionImage creates a pending image with a stable draft-scoped ID.
func NewDescriptionImage(filename, mimeType string, data []byte, width, height int) (DescriptionImage, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return DescriptionImage{}, fmt.Errorf("create description image id: %w", err)
	}
	return DescriptionImage{
		ID:       "img-" + hex.EncodeToString(random[:]),
		Filename: filename,
		MIMEType: mimeType,
		Data:     slices.Clone(data),
		Width:    width,
		Height:   height,
	}, nil
}

// ImageReferenceToken returns the textarea representation of an image occurrence.
func ImageReferenceToken(imageID, alt string) string {
	alt = strings.ReplaceAll(alt, `\`, `\\`)
	alt = strings.ReplaceAll(alt, "]", `\]`)
	return "![" + alt + "](" + imageReferencePrefix + imageID + ")"
}

// ParseDescriptionEditor validates editor text and reconciles its image references.
func ParseDescriptionEditor(text string, base Description) (Description, error) {
	if !base.Editable {
		return Description{}, errors.New("description is not editable: " + base.UnsupportedReason)
	}
	text = normalizeNewlines(text)
	images := slices.Clone(base.Images)
	queues := make(map[string][]DescriptionImageReference)
	for _, reference := range base.References {
		queues[reference.ImageID] = append(queues[reference.ImageID], reference)
	}
	used := make(map[string]int)
	references := make([]DescriptionImageReference, 0, len(base.References))
	lineNumber := 0
	for line := range strings.SplitSeq(text, "\n") {
		lineNumber++
		imageID, alt, matched, err := parseImageReference(line)
		if err != nil {
			return Description{}, fmt.Errorf("description line %d: %w", lineNumber, err)
		}
		if !matched {
			continue
		}
		if imageByID(images, imageID) == nil {
			return Description{}, fmt.Errorf("description line %d: unknown description image %q", lineNumber, imageID)
		}
		if strings.TrimSpace(alt) == "" {
			return Description{}, fmt.Errorf("description line %d: image alt text is required", lineNumber)
		}
		reference := DescriptionImageReference{ImageID: imageID, Alt: alt}
		if existing := queues[imageID]; used[imageID] < len(existing) {
			reference.Presentation = slices.Clone(existing[used[imageID]].Presentation)
		} else if len(existing) > 0 {
			reference.Presentation = copiedPresentation(existing[0].Presentation)
		}
		used[imageID]++
		references = append(references, reference)
	}
	return Description{
		EditorText: text,
		Editable:   true,
		Images:     images,
		References: references,
		RawADF:     slices.Clone(base.RawADF),
	}, nil
}

// ReferencedPendingImages returns pending images in first-reference order.
func (d Description) ReferencedPendingImages() []DescriptionImage {
	seen := make(map[string]bool)
	images := make([]DescriptionImage, 0)
	for _, reference := range d.References {
		if seen[reference.ImageID] {
			continue
		}
		seen[reference.ImageID] = true
		if image := imageByID(d.Images, reference.ImageID); image != nil && len(image.Data) > 0 && image.URL == "" && image.MediaID == "" {
			images = append(images, *image)
		}
	}
	return images
}

// MissingPendingImageData returns referenced images that cannot be uploaded or serialized.
func (d Description) MissingPendingImageData() []DescriptionImage {
	seen := make(map[string]bool)
	missing := make([]DescriptionImage, 0)
	for _, reference := range d.References {
		if seen[reference.ImageID] {
			continue
		}
		seen[reference.ImageID] = true
		if image := imageByID(d.Images, reference.ImageID); image != nil && len(image.Data) == 0 && image.URL == "" && image.MediaID == "" {
			missing = append(missing, *image)
		}
	}
	return missing
}

// WithUploadedImage records Jira attachment metadata and releases pending bytes.
func (d Description) WithUploadedImage(uploaded DescriptionImage) (Description, error) {
	index := slices.IndexFunc(d.Images, func(image DescriptionImage) bool { return image.ID == uploaded.ID })
	if index < 0 {
		return Description{}, fmt.Errorf("unknown description image %q", uploaded.ID)
	}
	d.Images = slices.Clone(d.Images)
	d.Images[index] = uploaded
	d.Images[index].Data = nil
	return d, nil
}

// WithoutImageData returns description metadata safe for ordinary issue caches.
func (d Description) WithoutImageData() Description {
	d.Images = slices.Clone(d.Images)
	for index := range d.Images {
		d.Images[index].Data = nil
	}
	return d
}

func descriptionFromADF(value any) Description {
	if value == nil {
		return PlainDescription("")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return unsupportedDescription(value, "description is not valid ADF")
	}
	var root adfNode
	if json.Unmarshal(raw, &root) != nil || root.Type != "doc" {
		return unsupportedDescription(value, "description root is not an ADF document")
	}
	lines := make([]string, 0, len(root.Content))
	images := make([]DescriptionImage, 0)
	references := make([]DescriptionImageReference, 0)
	for index, block := range root.Content {
		switch block.Type {
		case "paragraph":
			line, ok := plainParagraph(block)
			if !ok {
				return unsupportedDescription(value, fmt.Sprintf("paragraph %d contains unsupported rich text", index+1))
			}
			lines = append(lines, line)
		case "mediaSingle":
			image, reference, ok := descriptionImageBlock(block)
			if !ok {
				return unsupportedDescription(value, fmt.Sprintf("media block %d is not a supported image", index+1))
			}
			if existing := imageByID(images, image.ID); existing == nil {
				images = append(images, image)
			}
			references = append(references, reference)
			lines = append(lines, ImageReferenceToken(reference.ImageID, reference.Alt))
		default:
			return unsupportedDescription(value, fmt.Sprintf("ADF block %q is not editable", block.Type))
		}
	}
	return Description{
		EditorText: strings.Join(lines, "\n"),
		Editable:   true,
		Images:     images,
		References: references,
		RawADF:     raw,
	}
}

func descriptionToADF(description Description, omitPending bool) (map[string]any, error) {
	description, err := ParseDescriptionEditor(description.EditorText, description)
	if err != nil {
		return nil, err
	}
	content := make([]any, 0, strings.Count(description.EditorText, "\n")+1)
	referenceIndex := 0
	for line := range strings.SplitSeq(description.EditorText, "\n") {
		imageID, _, matched, err := parseImageReference(line)
		if err != nil {
			return nil, err
		}
		if matched {
			reference := description.References[referenceIndex]
			referenceIndex++
			image := imageByID(description.Images, imageID)
			if image == nil {
				return nil, fmt.Errorf("unknown description image %q", imageID)
			}
			pending := image.URL == "" && image.MediaID == ""
			if pending && omitPending {
				continue
			}
			if pending {
				return nil, fmt.Errorf("description image %q has not been uploaded", imageID)
			}
			node, err := descriptionImageADFNode(reference, *image)
			if err != nil {
				return nil, err
			}
			content = append(content, node)
			continue
		}
		paragraph := map[string]any{"type": "paragraph"}
		if line != "" {
			paragraph["content"] = []any{map[string]any{"type": "text", "text": line}}
		}
		content = append(content, paragraph)
	}
	return map[string]any{"type": "doc", "version": 1, "content": content}, nil
}

func descriptionImageADFNode(reference DescriptionImageReference, image DescriptionImage) (map[string]any, error) {
	if len(reference.Presentation) > 0 {
		var node map[string]any
		if err := json.Unmarshal(reference.Presentation, &node); err != nil {
			return nil, fmt.Errorf("restore description image presentation: %w", err)
		}
		content, _ := node["content"].([]any)
		if len(content) == 0 {
			return nil, errors.New("description image presentation has no media node")
		}
		media, _ := content[0].(map[string]any)
		attrs, _ := media["attrs"].(map[string]any)
		if attrs == nil {
			return nil, errors.New("description image presentation has no media attributes")
		}
		attrs["alt"] = reference.Alt
		return node, nil
	}
	attrs := map[string]any{
		"type": "external",
		"url":  image.URL,
		"alt":  reference.Alt,
	}
	if image.Width > 0 {
		attrs["width"] = image.Width
	}
	if image.Height > 0 {
		attrs["height"] = image.Height
	}
	return map[string]any{
		"type":  "mediaSingle",
		"attrs": map[string]any{"layout": "center"},
		"content": []any{map[string]any{
			"type":  "media",
			"attrs": attrs,
		}},
	}, nil
}

func plainParagraph(node adfNode) (string, bool) {
	if len(node.Marks) > 0 || len(node.Attrs) > 0 {
		return "", false
	}
	var builder strings.Builder
	for _, child := range node.Content {
		if child.Type != "text" || len(child.Marks) > 0 || len(child.Attrs) > 0 || strings.ContainsAny(child.Text, "\r\n") {
			return "", false
		}
		builder.WriteString(child.Text)
	}
	return builder.String(), true
}

func descriptionImageBlock(node adfNode) (DescriptionImage, DescriptionImageReference, bool) {
	if len(node.Content) == 0 || node.Content[0].Type != "media" {
		return DescriptionImage{}, DescriptionImageReference{}, false
	}
	media := node.Content[0]
	mediaType := firstADFAttr(media, "type")
	identity := ""
	image := DescriptionImage{
		Filename:   firstADFAttr(media, "alt", "title"),
		URL:        firstADFAttr(media, "url"),
		MediaID:    firstADFAttr(media, "id"),
		Collection: firstADFAttr(media, "collection"),
		Width:      asInt(media.Attrs["width"]),
		Height:     asInt(media.Attrs["height"]),
	}
	switch mediaType {
	case "external":
		if image.URL == "" {
			return DescriptionImage{}, DescriptionImageReference{}, false
		}
		identity = "external\x00" + image.URL
	case "file", "link":
		if image.MediaID == "" || image.Collection == "" {
			return DescriptionImage{}, DescriptionImageReference{}, false
		}
		identity = mediaType + "\x00" + image.MediaID + "\x00" + image.Collection
	default:
		return DescriptionImage{}, DescriptionImageReference{}, false
	}
	if image.Filename == "" {
		image.Filename = "image"
	}
	hash := sha256.Sum256([]byte(identity))
	image.ID = "img-" + hex.EncodeToString(hash[:6])
	presentation, err := json.Marshal(node)
	if err != nil {
		return DescriptionImage{}, DescriptionImageReference{}, false
	}
	reference := DescriptionImageReference{ImageID: image.ID, Alt: image.Filename, Presentation: presentation}
	return image, reference, true
}

func unsupportedDescription(value any, reason string) Description {
	raw, _ := json.Marshal(value)
	var root adfNode
	_ = json.Unmarshal(raw, &root)
	return Description{
		EditorText:        strings.TrimSpace(adfText(root)),
		Editable:          false,
		UnsupportedReason: reason,
		RawADF:            raw,
	}
}

func parseImageReference(line string) (imageID, alt string, matched bool, err error) {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, imageReferencePrefix) {
		return "", "", false, nil
	}
	if !strings.HasPrefix(line, "![") || !strings.HasSuffix(line, ")") {
		return "", "", false, errors.New("malformed description image reference")
	}
	closing := -1
	escaped := false
	for index := 2; index < len(line); index++ {
		switch {
		case escaped:
			escaped = false
		case line[index] == '\\':
			escaped = true
		case line[index] == ']' && index+1 < len(line) && line[index+1] == '(':
			closing = index
			index = len(line)
		}
	}
	if closing < 0 {
		return "", "", false, errors.New("malformed description image reference")
	}
	target := line[closing+2 : len(line)-1]
	if !strings.HasPrefix(target, imageReferencePrefix) {
		return "", "", false, errors.New("malformed description image reference target")
	}
	imageID = strings.TrimPrefix(target, imageReferencePrefix)
	if !imageIDRE.MatchString(imageID) {
		return "", "", false, errors.New("invalid description image id")
	}
	alt, err = unescapeImageAlt(line[2:closing])
	if err != nil {
		return "", "", false, err
	}
	return imageID, alt, true, nil
}

func unescapeImageAlt(value string) (string, error) {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' {
			builder.WriteByte(value[index])
			continue
		}
		index++
		if index >= len(value) || (value[index] != '\\' && value[index] != ']') {
			return "", errors.New("invalid image alt-text escape")
		}
		builder.WriteByte(value[index])
	}
	return builder.String(), nil
}

func imageByID(images []DescriptionImage, id string) *DescriptionImage {
	index := slices.IndexFunc(images, func(image DescriptionImage) bool { return image.ID == id })
	if index < 0 {
		return nil
	}
	return &images[index]
}

func copiedPresentation(raw []byte) []byte {
	var node map[string]any
	if json.Unmarshal(raw, &node) != nil {
		return slices.Clone(raw)
	}
	content, _ := node["content"].([]any)
	if len(content) > 0 {
		media, _ := content[0].(map[string]any)
		attrs, _ := media["attrs"].(map[string]any)
		delete(attrs, "occurrenceKey")
	}
	copy, err := json.Marshal(node)
	if err != nil {
		return slices.Clone(raw)
	}
	return copy
}

func normalizeNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}
