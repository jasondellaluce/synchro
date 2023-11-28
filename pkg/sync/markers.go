package sync

type CommitMarker string

const (
	// IgnoreCommitMarker is a keyword that can be used for signaling that a given
	// commit should be ignored during the scanning process.
	CommitMarkerIgnore CommitMarker = "SYNC_IGNORE"

	// CommitMarkerConflictSkip is a keyword that can be used for signaling that a given
	// commit should be skipped in case of a merge conflict
	CommitMarkerConflictSkip CommitMarker = "SYNC_CONFLICT_SKIP"

	// CommitMarkerConflictApply is a keyword that can be used for signaling that a given
	// commit should be always applied in case of a merge conflict. In case
	// of content conflict markers, the commit's markers are chosen.
	CommitMarkerConflictApply CommitMarker = "SYNC_CONFLICT_APPLY"
)

// A collection of all commit markers available
var AllCommitMarkers = []CommitMarker{
	CommitMarkerIgnore,
	CommitMarkerConflictSkip,
	CommitMarkerConflictApply,
}

func (c CommitMarker) String() string {
	return string(c)
}

func (c CommitMarker) Description() string {
	return "TODO"
}
