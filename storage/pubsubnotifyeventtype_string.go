// Code generated by "stringer -type PubSubNotifyEventType"; DO NOT EDIT.

package storage

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ObjectFinalize-0]
	_ = x[ObjectMetaDataUpdate-1]
	_ = x[ObjectDelete-2]
	_ = x[ObjectArchive-3]
}

const _PubSubNotifyEventType_name = "ObjectFinalizeObjectMetaDataUpdateObjectDeleteObjectArchive"

var _PubSubNotifyEventType_index = [...]uint8{0, 14, 34, 46, 59}

func (i PubSubNotifyEventType) String() string {
	if i < 0 || i >= PubSubNotifyEventType(len(_PubSubNotifyEventType_index)-1) {
		return "PubSubNotifyEventType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _PubSubNotifyEventType_name[_PubSubNotifyEventType_index[i]:_PubSubNotifyEventType_index[i+1]]
}
