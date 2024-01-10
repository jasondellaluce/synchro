package sync

type CommitMarker string

const (
	// IgnoreCommitMarker is a keyword that can be used for signaling that a given
	// commit should be ignored during the sync process.
	CommitMarkerIgnore CommitMarker = "SYNC_IGNORE"

	// CommitMarkerConflictSkip is a keyword that can be used for signaling that a given
	// commit should be skipped in case of a merge conflict
	CommitMarkerConflictSkip CommitMarker = "SYNC_CONFLICT_SKIP"

	// CommitMarkerConflictApply is a keyword that can be used for signaling that a given
	// commit should be always applied in case of a merge conflict. In case
	// of content conflict markers, the commit's markers are chosen.
	CommitMarkerConflictApply CommitMarker = "SYNC_CONFLICT_APPLY"
)

// AllCommitMarkers is a collection of all commit markers supported
var AllCommitMarkers = []CommitMarker{
	CommitMarkerIgnore,
	CommitMarkerConflictSkip,
	CommitMarkerConflictApply,
}

func (c CommitMarker) String() string {
	return string(c)
}

func (c CommitMarker) Description() string {
	switch c {
	case CommitMarkerIgnore:
		return "The commit should be ignored during the sync"
	case CommitMarkerConflictSkip:
		return "In case of a merge conflict, the conflicting changes of the commit should be skipped"
	case CommitMarkerConflictApply:
		return "In case of a merge conflict, the conflicting changes of the commit should be forcefully applied"
	default:
		panic("CommitMarker.Description invoked on invalid instance")
	}
}
