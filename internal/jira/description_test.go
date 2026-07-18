package jira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDescriptionReferencesShareOneImage(t *testing.T) {
	image := DescriptionImage{
		ID:       "img-1",
		Filename: "screen.png",
		URL:      "https://example.atlassian.net/rest/api/3/attachment/content/1",
		Width:    800,
		Height:   600,
	}
	token := ImageReferenceToken(image.ID, "Login error")
	description, err := ParseDescriptionEditor(token+"\nBetween\n"+token, Description{
		EditorText: token,
		Editable:   true,
		Images:     []DescriptionImage{image},
		References: []DescriptionImageReference{{ImageID: image.ID, Alt: "Login error"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(description.References) != 2 || len(description.Images) != 1 {
		t.Fatalf("description = %#v", description)
	}
	adf, err := descriptionToADF(description, false)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(adf)
	if got := strings.Count(string(data), image.URL); got != 2 {
		t.Fatalf("image occurrences = %d, ADF = %s", got, data)
	}
}

func TestDescriptionRejectsMalformedAndUnknownReferences(t *testing.T) {
	base := Description{Editable: true, Images: []DescriptionImage{{ID: "img-1"}}}
	for _, text := range []string{
		`![missing close](jiratui-image:img-1`,
		`![](jiratui-image:img-1)`,
		`![Alt](jiratui-image:img-missing)`,
	} {
		if _, err := ParseDescriptionEditor(text, base); err == nil {
			t.Fatalf("ParseDescriptionEditor(%q) succeeded", text)
		}
	}
}

func TestInitialDescriptionOmitsPendingImageReferences(t *testing.T) {
	image := DescriptionImage{ID: "img-1", Filename: "screen.png", Data: []byte("png")}
	token := ImageReferenceToken(image.ID, image.Filename)
	description, err := ParseDescriptionEditor("Before\n"+token+"\nAfter", Description{
		Editable: true,
		Images:   []DescriptionImage{image},
	})
	if err != nil {
		t.Fatal(err)
	}
	adf, err := descriptionToADF(description, true)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(adf)
	if strings.Contains(string(data), "jiratui-image") || strings.Contains(string(data), "screen.png") {
		t.Fatalf("pending reference leaked into ADF: %s", data)
	}
}

func TestDescriptionImageTokenEscapesAltText(t *testing.T) {
	token := ImageReferenceToken("img-1", `Login ] path \\ failure`)
	description, err := ParseDescriptionEditor(token, Description{
		Editable: true,
		Images:   []DescriptionImage{{ID: "img-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := description.References[0].Alt; got != `Login ] path \\ failure` {
		t.Fatalf("alt = %q", got)
	}
}
