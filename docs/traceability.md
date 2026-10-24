# Traceability

This page maps every rule in [`spec.md`](spec.md) to the test or tests that
enforce it. `AGENTS.md` section 8 requires this table to be updated in the same
change that adds or modifies a rule's behaviour.

**Status:** M0–M2 are done; rules covered by those milestones are marked
**covered**. Remaining rules are open gaps with the milestone that will cover
them. A gap is not "done"; do not mark a milestone complete while one of its
rules is still a gap.

## How to use this page

- When you write a test for a rule, name or comment it with the rule ID, for
  example `TestItemTitleDefaultsToFirstAttachment_I2`.
- Replace the `—` in the "Test(s)" column with the test name and change the
  status to **covered**.
- If a rule needs more than one test (happy path and failures), list them all.

## Items (`I-*`)

| Rule | Test(s) | Status |
|---|---|---|
| I-1 | `TestCreateItemFlow_I1_I2_I4` | covered |
| I-2 | `TestCleanTitle_RequiresTitleWithoutAttachments_I2`, `TestCleanTitle_DefaultsToFirstAttachment_I2`, `TestItemService_Create_ValidatesTitle_I2`, `TestCreateItem_RejectsEmptyTitle_I2` | covered |
| I-3 | `TestItemService_Update_SetsUpdatedAt_I5` (a defaulted title is stored as an ordinary, editable title) | covered |
| I-4 | `TestCreateItemFlow_I1_I2_I4` (a note-only item saves) | covered |
| I-5 | `TestStore_SetFavorite_DoesNotTouchUpdatedAt_I5`, `TestItemService_Update_SetsUpdatedAt_I5` | covered |
| I-6 | `TestEditItemFlow_I6` | covered |
| I-7 | — | gap (M5) |
| I-8 | — | gap (runtime guard, M9) |
| I-9 | `TestCreateItemFlow_I1_I2_I4` (notes are stored as ordinary text) | covered |

## Attachments (`A-*`)

| Rule | Test(s) | Status |
|---|---|---|
| A-1 | `TestStore_CreateItemWithAttachments_A1_A3_R2`, `TestCreateItemWithAttachments_A1_A3_A4_A5_A6` | covered |
| A-2 | `TestCleanLinkURL_A2`, `TestCreateItem_RejectsBadScheme_A2` | covered |
| A-3 | `TestStore_CreateItemWithAttachments_A1_A3_R2`, `TestItemService_Create_StoresFileWithSHA256_A3_A4`, `TestSave_StoresByIDNotName_A3` | covered |
| A-4 | `TestItemService_Create_StoresFileWithSHA256_A3_A4`, `TestSave_ComputesSHA256AndSize_A4` | covered |
| A-5 | `TestCreateItemWithAttachments_A1_A3_A4_A5_A6` | covered |
| A-6 | `TestCreateItemWithAttachments_A1_A3_A4_A5_A6` | covered |
| A-7 | `TestDownloadAttachment_A7_A9`, `TestStore_AttachmentByID_A7` | covered |
| A-8 | `TestSave_RejectsPathTraversalName`, `TestCreateItemWithAttachments_A1_A3_A4_A5_A6` (files are stored by id, never by name) | covered |
| A-9 | — | gap (M5) |

## Tags and favorites (`T-*`, `F-*`)

| Rule | Test(s) | Status |
|---|---|---|
| T-1 | `TestItemService_Create_StoresTagsCaseInsensitively_T3_T5`, `TestStore_CreateAndGetItem_I2` | covered |
| T-2 | `TestStore_SuggestTags_T2` | covered |
| T-3 | `TestCleanTagName_T3`, `TestItemService_Create_StoresTagsCaseInsensitively_T3_T5` | covered |
| T-4 | `TestTagKey_IsCaseInsensitive_T4`, `TestStore_TagReuseIsCaseInsensitive_T4` | covered |
| T-5 | `TestItemService_Create_StoresTagsCaseInsensitively_T3_T5`, `TestStore_UpdateReplacesTags_T5` | covered |
| T-6 | `TestStore_ActiveTags_ExcludesTrashedOnlyTags_T6` | covered |
| F-1 | `TestFavoriteToggle_F1` | covered |
| F-2 | `TestFavoriteToggle_F1` | covered |
| F-3 | `TestStore_ListItems_FiltersAndSort_B1_B2_B3` (favorites combine with other filters) | covered |

## Browsing (`B-*`)

| Rule | Test(s) | Status |
|---|---|---|
| B-1 | `TestStore_ListItems_FiltersAndSort_B1_B2_B3` | covered |
| B-2 | `TestStore_ListItems_FiltersAndSort_B1_B2_B3` | covered |
| B-3 | `TestItemList_FiltersAndCount_B7` | covered |
| B-4 | `TestStore_ListItems_TagFilterRequiresAll_B4` | covered |
| B-5 | `TestStore_ListItems_AttachmentKindFilters_B5` | covered |
| B-6 | `TestStore_ActiveTags_ExcludesTrashedOnlyTags_T6`, `TestStore_ListItems_FiltersAndSort_B1_B2_B3` | covered |
| B-7 | `TestItemList_FiltersAndCount_B7`, `TestStore_ListItems_Pagination_B7`, `TestItemService_List_DefaultsAndClampsPageSize_B7` | covered |

## Search (`S-*`)

| Rule | Test(s) | Status |
|---|---|---|
| S-1 | `TestSearch_CoversTitleDescriptionNoteTags_S1`, `TestSearch_ExcludesURLsLabelsAndFileNames_S1` | covered |
| S-2 | `TestSearch_PrefixCaseInsensitiveAllWordsRequired_S2`, `TestSearchFlow_S2_S5` | covered |
| S-3 | `TestSearch_TitleRanksAboveBody_S3` | covered |
| S-4 | `TestSearch_MarkdownSymbolsDoNotAffectMatching_S4` | covered |
| S-5 | `TestSearch_CombinesWithFilters_S5` | covered |
| S-6 | `TestSearch_SavedChangeIsImmediatelySearchable_S6` | covered |

## Duplicates (`D-*`, `R-*`)

| Rule | Test(s) | Status |
|---|---|---|
| D-1 | — | gap (M6) |
| D-2 | — | gap (M6) |
| D-3 | — | gap (M6) |
| D-4 | — | gap (M6) |
| D-5 | — | gap (M6) |
| R-1 | — | gap (M6) |
| R-2 | `TestNormalizeURL_R2`, `TestNormalizeURL_FragmentOnly` | covered (matching; report lands in M6) |
| R-3 | — | gap (M6) |
| R-4 | — | gap (M6) |
| R-5 | — | gap (M6) |
| R-6 | — | gap (M6) |

## Trash (`X-*`)

| Rule | Test(s) | Status |
|---|---|---|
| X-1 | — | gap (M5) |
| X-2 | — | gap (M5) |
| X-3 | — | gap (M5) |
| X-4 | — | gap (M5) |
| X-5 | — | gap (M5) |
| X-6 | — | gap (M5) |
| X-7 | — | gap (M5) |
| X-8 | — | gap (M5) |
| X-9 | — | gap (M5) |
| X-10 | — | gap (M5) |
| X-11 | — | gap (M5) |
| X-12 | — | gap (M5) |
| X-13 | — | gap (M5) |

## Tag management (`M-*`)

| Rule | Test(s) | Status |
|---|---|---|
| M-1 | — | gap (M7) |
| M-2 | — | gap (M7) |
| M-3 | — | gap (M7) |
| M-4 | — | gap (M7) |
| M-5 | — | gap (M7) |

## Markdown (`V-*`)

| Rule | Test(s) | Status |
|---|---|---|
| V-1 | `TestMarkdown_RendersStandardSyntax_V1` | covered |
| V-2 | `TestMarkdown_RawHTMLNotRendered_V2`, `TestMarkdown_RejectsJavascriptLinks_V2`, `TestItemDetail_RawHTMLNeverRendered_V2` | covered |
| V-3 | `TestMarkdown_ImagesBecomeLinks_V3` | covered |
| V-4 | `TestMarkdown_LinksOpenInNewTab_V4` | covered |

## Authentication (`U-*`)

| Rule | Test(s) | Status |
|---|---|---|
| U-1 | `TestSetupFlow_U1_U2_U5`, `TestService_SetupAndLogin_U1_U4` | covered |
| U-2 | `TestValidateNewPassword_U2`, `TestSetupFlow_U1_U2_U5` | covered |
| U-3 | `TestUnauthenticatedRoutesRejected_U3` (enumerates every registered route) | covered |
| U-4 | `TestService_SetupAndLogin_U1_U4`, `TestLoginFlow_U4` | covered |
| U-5 | `TestSetupFlow_U1_U2_U5` (cookie has no Expires/Max-Age, HttpOnly, SameSite=Lax) | covered |
| U-6 | `TestService_Logout_U6`, `TestLogout_U6` | covered |
| U-7 | `TestService_ChangePassword_U7`, `TestChangePassword_U7` | covered |
| U-8 | `TestHashPassword_RoundTrip_U8`, `TestHashPassword_SaltsDiffer`, `TestVerify_RejectsMalformedHashes` | covered |
| U-9 | `TestService_LoginRejectsWrongPassword_U9`, `TestLoginThrottle_U9`, `TestThrottlePolicy_BlockDoubling_U9`, `TestThrottler_SuccessResets_U9` | covered |
| U-10 | `TestService_ResetPassword_U10`, `TestPerformReset_U10`, `TestPerformReset_RejectsMismatch_U10` | covered |
| U-11 | — | out of scope (deployment, spec section 1.3) |

## Export and import (`E-*`)

| Rule | Test(s) | Status |
|---|---|---|
| E-1 | — | gap (M8) |
| E-2 | — | gap (M8) |
| E-3 | — | gap (M8) |
| E-4 | — | gap (M8) |
| E-5 | — | gap (M8) |
| E-6 | — | gap (M8) |
| E-7 | — | gap (M8) |
| E-8 | — | gap (M8) |
| E-9 | — | gap (M8) |
| E-10 | — | gap (M8) |

## Scale (`N-*`)

| Rule | Test(s) | Status |
|---|---|---|
| N-1 | — | gap (limits documented in M9) |
| N-2 | — | gap (`make bench`, M9) |

## Infrastructure tests (not spec rules)

These tests protect the foundations the spec rules depend on.

| Area | Test | What it proves |
|---|---|---|
| Configuration | `TestLoad_Defaults`, `TestLoad_EnvOverridesDefaults`, `TestLoad_FlagsOverrideEnv`, `TestLoad_InvalidInput` | Flags beat environment variables, which beat defaults; invalid input fails at startup. |
| SQLite setup | `TestOpen_AppliesPragmas`, `TestOpen_EnforcesForeignKeys` | Foreign keys on, WAL journal, busy timeout set. |
| File safety | `TestOpen_CreatesPrivateFile`, `TestOpen_PathWithSpaces` | The database file is `0600`; unusual paths work. |
| Migrations | `TestMigrator_AppliesInOrderAndIsIdempotent`, `TestMigrator_RollsBackFailedMigration`, `TestMigrator_RejectsMalformedNames` | Ordered, transactional, idempotent, strict naming. |
| FTS5 | `TestFTS5_AvailableAndUsable` | The driver supports FTS5, prefix matching, case folding and bm25 ranking (see ADR 0002). |
| Attachments | `TestCreateItemWithAttachments_A1_A3_A4_A5_A6`, `TestDownloadAttachment_A7_A9`, `TestSave_*`, `TestItemService_Create_StoresFileWithSHA256_A3_A4` | Links and files are added through one form; files stream to disk with SHA-256, are stored by attachment id, and download under the original name with safe headers. |
| Items and tags | `TestStore_CreateAndGetItem_I2`, `TestStore_ListItems_*`, `TestItemService_*`, `TestCreateItemFlow_*`, `TestEditItemFlow_I6`, `TestFavoriteToggle_F1` | Item create/edit/list, tag resolution, filtering, sorting and pagination work against a real SQLite database and over HTTP. |
| Search safety | `TestSearch_SyntaxInjectionIsInert`, `TestBuildMatch` | No user input reaches FTS5 as syntax; every operator-like query is treated as literal text. |
| Markdown safety | `TestMarkdown_*`, `TestItemDetail_RawHTMLNeverRendered_V2` | Notes render standard Markdown while raw HTML, event handlers and `javascript:` URLs are removed (V-1..V-4). |
| Authentication | `TestSetupFlow_*`, `TestLoginFlow_*`, `TestLoginThrottle_U9`, `TestChangePassword_U7`, `TestLogout_U6` | First-run setup, sessions, cookie attributes, throttling, password change and logout work end to end against a real database. |
| HTTP surface | `TestHealthz_ReturnsOK`, `TestHealthz_OnlyAllowsGET`, `TestUnknownRoute_NotFound`, `TestHealthz_RevealsNoData`, `TestSecurityHeaders_AppliedToEveryResponse` | `/healthz` is minimal and read-only, and security headers are on every response. |
