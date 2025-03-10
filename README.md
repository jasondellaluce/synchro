# synchro

_Next-gen tooling for keeping in sync private forks of open source repositories_

## Installing

```
curl -sSL https://raw.githubusercontent.com/jasondellaluce/synchro/main/install.sh | bash
```

## Commit Markers

Commit markers are keywords that can be annotated either in the body message of a commit, or in one or more GitHub comments relative to a commit. They can be used to influence the behavior of the `synchro` tool when scanning a given commit during a fork sync.

|        MARKER         |                                           DESCRIPTION                                           |
|-----------------------|-------------------------------------------------------------------------------------------------|
| `SYNC_IGNORE`         | The commit should be ignored during the sync                                                    |
| `SYNC_CONFLICT_SKIP`  | In case of a merge conflict, the conflicting changes of the commit should be skipped            |
| `SYNC_CONFLICT_APPLY` | In case of a merge conflict, the conflicting changes of the commit should be forcefully applied |

## Merge Conflict Recovery

The `synchro` tools supports automatic recovery from many scenarios of git merge conflict that could arise when picking a commit during a fork sync. The default recovery strategy of the tool can be influenced by the markers annotated on each commit.

|    CONFLICT     |                                               DESCRIPTION                                                |                                                                                                                                                                                                                    RECOVERY                                                                                                                                                                                                                     |
|-----------------|----------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `content`       | A file has been modified both in upstream and downstream in similar locations but with different changes | Conflict markers are solved by accepting the upstream modifications if the commit is marked with `SYNC_CONFLICT_SKIP`, and by accepting the downstream ones if marked with `SYNC_CONFLICT_APPLY`. By default, conflicts are tentatively solved using the cache provided of the `synchro conflict` commands (powerded by `git rerere`). If failing, the recovery attempt is aborted and guidance on the required manual intervention is provided |
| `delete-modify` | A file has both been deleted upstream and modified downstream                                            | The file is preserved with the new modifications if the commit is marked with `SYNC_CONFLICT_APPLY`, and deleted otherwise                                                                                                                                                                                                                                                                                                                      |
| `delete-rename` | A file has both been deleted upstream and renamed downstream                                             | The file is preserved with the new name if the commit is marked with `SYNC_CONFLICT_APPLY`, and deleted otherwise                                                                                                                                                                                                                                                                                                                               |
| `rename-rename` | A file has been renamed both upstream and downstream, but with different names                           | The file is renamed with the upstream if the commit is marked with `SYNC_CONFLICT_SKIP`, and with the downstream name otherwise                                                                                                                                                                                                                                                                                                                 |
| `rename-delete` | A file has both been renamed upstream and deleted downstream                                             | The file is preserved with the new name if the commit is marked with `SYNC_CONFLICT_SKIP`, and deleted otherwise                                                                                                                                                                                                                                                                                                                                |
| `modify-delete` | A file has both been modified upstream and deleted downstream                                            | The file is preserved with the new modifications if the commit is marked with `SYNC_CONFLICT_SKIP`, and deleted otherwise                                                                                                                                                                                                                                                                                                                       |
