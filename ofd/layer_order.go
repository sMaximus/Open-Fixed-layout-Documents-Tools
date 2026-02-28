package ofd

import (
	"sort"
	"strconv"
	"strings"
)

type layerObjectType int

const (
	layerObjectPath layerObjectType = iota
	layerObjectImage
	layerObjectText
)

type layerObjectRef struct {
	objType layerObjectType
	order   int
	id      int
	hasID   bool

	path  *PathObject
	image *ImageObject
	text  *TextObject
}

// orderedLayerObjects merges all layer objects and applies ID-based ordering.
// In many OFD files, decoration lines (underline/strikethrough) are PathObject
// nodes placed immediately after TextObject nodes and should be rendered later.
func orderedLayerObjects(layer *Layer) []layerObjectRef {
	total := len(layer.PathObjects) + len(layer.ImageObjects) + len(layer.TextObjects)
	objs := make([]layerObjectRef, 0, total)
	order := 0

	for i := range layer.PathObjects {
		obj := &layer.PathObjects[i]
		id, hasID := parseNumericObjectID(obj.ID)
		objs = append(objs, layerObjectRef{
			objType: layerObjectPath,
			order:   order,
			id:      id,
			hasID:   hasID,
			path:    obj,
		})
		order++
	}

	for i := range layer.ImageObjects {
		obj := &layer.ImageObjects[i]
		id, hasID := parseNumericObjectID(obj.ID)
		objs = append(objs, layerObjectRef{
			objType: layerObjectImage,
			order:   order,
			id:      id,
			hasID:   hasID,
			image:   obj,
		})
		order++
	}

	for i := range layer.TextObjects {
		obj := &layer.TextObjects[i]
		id, hasID := parseNumericObjectID(obj.ID)
		objs = append(objs, layerObjectRef{
			objType: layerObjectText,
			order:   order,
			id:      id,
			hasID:   hasID,
			text:    obj,
		})
		order++
	}

	sort.SliceStable(objs, func(i, j int) bool {
		a := objs[i]
		b := objs[j]
		if a.hasID && b.hasID && a.id != b.id {
			return a.id < b.id
		}
		return a.order < b.order
	})

	return objs
}

func parseNumericObjectID(id string) (int, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
