package sync

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/jasondellaluce/synchro/pkg/utils"
)

type conflictSuggestionInfo struct {
	UpstreamOrg       string
	UpstreamRepo      string
	UpstreamRef       string
	ForkOrg           string
	ForkRepo          string
	ConflictCommitSHA string
	BranchName        string
}

func (i *conflictSuggestionInfo) ProjectRepo() string {
	return utils.ProjectRepo
}

func (i *conflictSuggestionInfo) PackageName() string {
	return utils.PackageName
}

func formatConflictSuggestion(t *template.Template, info *conflictSuggestionInfo) string {
	b := bytes.Buffer{}
	err := t.Execute(&b, info)
	if err != nil {
		panic("failure when executing template: " + err.Error())
	}
	return b.String()
}

// todo: add suggestions for SYNC_IGNORE
// todo: support new markers such as SYNC_USEFORK, or SYNC_USEUPSTREAM
var contentConflictSuggestion = template.Must(template.New("contentConflictSuggestion").Parse(strings.TrimSpace(`
Issue context:

* A merge conflict occurred and can't be resolved automatically
* Upstream base ref: https://github.com/{{ .UpstreamOrg }}/{{ .UpstreamRepo }}/tree/{{ .UpstreamRef}}
* Conflicting commit: https://github.com/{{ .ForkOrg }}/{{ .ForkRepo }}/commit/{{ .ConflictCommitSHA }}
* In-progress sync branch: https://github.com/{{ .ForkOrg }}/{{ .ForkRepo }}/tree/{{ .BranchName }}

Action items:

1. Make sure to have installed both ` + "`" + `git` + "`" + ` and ` + "`" + `synchro` + "`" + ` ({{ .ProjectRepo }}):
   ` + "`" + `go install {{ .PackageName }}@latest` + "`" + `
2. Checkout fork repo and cd into it:
   ` + "`" + `cd /tmp && git clone git@github.com:{{ .ForkOrg }}/{{ .ForkRepo }}.git && cd {{ .ForkRepo }}` + "`" + `
3. Make sure ` + "`" + `git rerere` + "`" + ` is enabled in the repo and pull latest cached resolutions:
   ` + "`" + `git config rerere.enabled true` + "`" + `
   ` + "`" + `synchro conflict pull` + "`" + `
4. Checkout unfinished sync branch:
   ` + "`" + `git fetch origin` + "`" + `
   ` + "`" + `git checkout {{ .BranchName }}` + "`" + `
5. Apply the conflicting commit, solve the conflict manually, and commit it:
   ` + "`" + `git cherry-pick {{ .ConflictCommitSHA }}` + "`" + `
   ... solve conflicts manually, then stage all changes...
   ` + "`" + `git cherry-pick --continue` + "`" + `
6. Update fork's conflict resolution cache so that this won't be asked again:
   ` + "`" + `synchro conflict push` + "`" + `
`)))
