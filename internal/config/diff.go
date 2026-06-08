package config

import (
	"sort"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// diffConfigMaps computes the field-level diff between two config snapshots.
// Each snapshot maps field path to value. A field present only in updated is
// ADDED; present only in old is REMOVED; present in both with a different value
// is MODIFIED. Unchanged fields are omitted. The result is sorted by field path
// for deterministic output.
//
// This mirrors sdk/tools/diff.Compare but is reimplemented here because the
// server must not import the SDK (separate module).
func diffConfigMaps(old, updated map[string]string) []*pb.FieldDiff {
	paths := make(map[string]struct{}, len(old)+len(updated))
	for p := range old {
		paths[p] = struct{}{}
	}
	for p := range updated {
		paths[p] = struct{}{}
	}

	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	diffs := make([]*pb.FieldDiff, 0, len(sorted))
	for _, p := range sorted {
		oldVal, inOld := old[p]
		newVal, inNew := updated[p]

		switch {
		case inOld && !inNew:
			diffs = append(diffs, &pb.FieldDiff{
				FieldPath:  p,
				ChangeType: pb.ChangeType_CHANGE_TYPE_REMOVED,
				OldValue:   oldVal,
			})
		case !inOld && inNew:
			diffs = append(diffs, &pb.FieldDiff{
				FieldPath:  p,
				ChangeType: pb.ChangeType_CHANGE_TYPE_ADDED,
				NewValue:   newVal,
			})
		case oldVal != newVal:
			diffs = append(diffs, &pb.FieldDiff{
				FieldPath:  p,
				ChangeType: pb.ChangeType_CHANGE_TYPE_MODIFIED,
				OldValue:   oldVal,
				NewValue:   newVal,
			})
		}
	}

	return diffs
}
