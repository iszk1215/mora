# `/api/tracker` 設計書 (v13)

- **v7**: 二重認証（Session + API Key）設計 + フロントエンド設計を追加
- **v8**: ユーザー権限管理（track_member/track_like）、スーパーユーザー、フロントエンド閲覧/編集分離
- **v9**: トラッカー作成ページ（TrackCreate）+ visibility 作成時指定必須化、アバタードロップダウンに Create Tracker 追加
- **v10**: ユーザー認証再設計 — `user` + `user_auth` テーブル分割、login/signup フロー分離、`FindOrCreate` 廃止、raw API によるプロバイダユーザー情報取得
- **v11**: `user_password` テーブル分割
- **v11+**: visibility による読み取り権限追加、PATCH/preview エンドポイント追加、ページネーション対応、API Key 認証を server パッケージに移管、編集/読み取りミドルウェア分離
- **v12**: tracker type 導入（`tracker`/`coverage`）、`tracker_coverage` テーブル追加、カバレッジ統合、フロントエンド表示分岐
- **v13**: `chart_config`/`config` フィールド追加、PATCH /series エンドポイント追加、フロントエンドに ECharts チャートコンポーネント・coverage 専用ビュー追加、全 v12 機能の完了を反映

## 概要

リポジトリに依存しない時系列データ追跡エンドポイント。既存の UDM (User Defined Metrics) と同様の概念だが、`repo_id` を持たずグローバルに利用できる。

- **パッケージ**: `tracker`
- **テーブル**: `tracker`, `tracker_series`, `tracker_value`, `tracker_member`, `tracker_like`, `tracker_coverage`
- **Go ファイル**: `tracker/store.go`, `tracker/handler.go`, `tracker/service.go`, `tracker/provider.go`
- **フロントエンド**: `frontend/src/tracker.tsx`, `frontend/src/tracker_coverage.tsx`, `frontend/src/chart.tsx`
- **既存ファイルの変更**: `server/server.go`（マウント + middleware）, `server/session.go`, `coverage/coverage_store.go`（`Timeline()` 追加）
- **CLIは実装しない**
- **OpenAPI**: swaggo コメント付き

---

## データモデル

### テーブル定義

```sql
CREATE TABLE IF NOT EXISTS tracker (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    visibility TEXT NOT NULL DEFAULT 'private',
    type TEXT NOT NULL DEFAULT 'tracker',   -- 'tracker' | 'coverage'
    chart_config TEXT NOT NULL DEFAULT '{}'  -- JSON: x_axis_label, y_axis_label, area, show_legend, y_max
);

CREATE TABLE IF NOT EXISTS tracker_series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    data_type TEXT NOT NULL DEFAULT 'float',
    config TEXT NOT NULL DEFAULT '{}',       -- JSON: value_format
    UNIQUE(tracker_id, name)
);

CREATE TABLE IF NOT EXISTS tracker_value (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES tracker_series(id) ON DELETE CASCADE,
    time DATETIME NOT NULL,
    value REAL NOT NULL,
    UNIQUE(series_id, time)
);

CREATE TABLE IF NOT EXISTS tracker_member (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    role     TEXT NOT NULL DEFAULT 'editor',   -- 'owner' | 'editor'
    PRIMARY KEY (user_id, tracker_id)
);

CREATE TABLE IF NOT EXISTS tracker_like (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    tracker_id INTEGER NOT NULL REFERENCES tracker(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, tracker_id)
);

CREATE TABLE IF NOT EXISTS tracker_coverage (
    tracker_id INTEGER PRIMARY KEY REFERENCES tracker(id) ON DELETE CASCADE,
    repo_id INTEGER NOT NULL,
    FOREIGN KEY (repo_id) REFERENCES repository(id) ON DELETE CASCADE
);
```

### 時刻精度について

`DATETIME` + Go の `time.Time` を使用する。

`go-sqlite3` ドライバは `time.Time` を ISO8601 文字列（例: `"2024-01-01T12:00:00.123456Z"`）として TEXT 保存する。このため SQLite の `DATETIME` 宣言でも**ナノ秒精度を正確に保持できる**。スキーマ変更不要。

### 階層構造

```
user
  ├── tracker_member (user_id, tracker_id, role)     — ON DELETE CASCADE
  ├── tracker_like   (user_id, tracker_id)           — ON DELETE CASCADE
  └── tracker (id, name, visibility, type, chart_config)
        ├── tracker_series (id, tracker_id, name, data_type, config)   — ON DELETE CASCADE
        │    └── tracker_value (id, series_id, time, value)  — ON DELETE CASCADE
        └── tracker_coverage (tracker_id PK, repo_id)          — ON DELETE CASCADE
```

type=`tracker`: tracker → series → values の通常の時系列データ
type=`coverage`: tracker → tracker_coverage（repo 参照）。series/values は持たない。preview は coverage store から取得

### 初期化時の移行

`initialize()` 内で旧 `track_*` テーブル（`track`, `track_series`, `track_value`, `track_member`, `track_like`）が存在すれば削除される。現行テーブル名は `tracker_*` で統一。

### data_type

| 値 | 意味 | グラフ表示のヒント |
|----|------|-------------------|
| `"int"` | 整数値 | 軸ラベルを整数表示 |
| `"float"`（default） | 浮動小数点数 | 軸ラベルを小数表示 |

保存はどちらも SQLite REAL 型（IEEE 64-bit double）。int64 は 2^53 未満を想定。表示桁数の調整は UI 側（フロントエンド）の責務。

---

## Go モデル (handler.go に定義)

### モデル

```go
type TrackerModel struct {
    Id          int64  `json:"id"         db:"id"`
    Name        string `json:"name"       db:"name"`
    Visibility  string `json:"visibility" db:"visibility"`
    Type        string `json:"type"       db:"type"` // "tracker" | "coverage"
    RepoID      *int64 `json:"repo_id,omitempty" db:"repo_id"`
    ChartConfig string `json:"chart_config" db:"chart_config"` // JSON
}

type SeriesModel struct {
    Id        int64  `json:"id"         db:"id"`
    TrackerId int64  `json:"tracker_id" db:"tracker_id"`
    Name      string `json:"name"       db:"name"`
    DataType  string `json:"data_type"  db:"data_type"`
    Config    string `json:"config"     db:"config"` // JSON: value_format
}

type ValueModel struct {
    Id        int64     `db:"id"`
    SeriesId  int64     `db:"series_id"`
    Timestamp time.Time `json:"time"  db:"time"`
    Value     float64   `json:"value" db:"value"`
}
```

### リクエスト型

```go
type CreateTrackerRequest struct {
    Name        string  `json:"name"`
    Visibility  string  `json:"visibility"` // required: "public"|"unlisted"|"private"
    Type        string  `json:"type"`       // "tracker" | "coverage", defaults to "tracker"
    RepoID      *int64  `json:"repo_id"`    // required if type="coverage"
    ChartConfig *string `json:"chart_config"`
}

type CreateSeriesRequest struct {
    Name     string  `json:"name"`
    DataType string  `json:"data_type"` // "int" or "float", default "float"
    Config   *string `json:"config"`    // JSON: value_format
}

type CreateValueRequest struct {
    Timestamp time.Time `json:"time"`
    Value     float64   `json:"value"`
}

type PatchTrackerRequest struct {
    Visibility  *string `json:"visibility"`
    ChartConfig *string `json:"chart_config"`
}

type PatchSeriesRequest struct {
    Name     *string `json:"name"`
    DataType *string `json:"data_type"`
    Config   *string `json:"config"`
}
```

### レスポンス型（swaggo からの参照用に公開）

```go
type TrackerResponse struct {
    Id          int64  `json:"id"`
    Name        string `json:"name"`
    Visibility  string `json:"visibility"` // "public" | "unlisted" | "private"
    Type        string `json:"type"`       // "tracker" | "coverage"
    RepoID      *int64 `json:"repo_id,omitempty"`
    ChartConfig string `json:"chart_config"`
    Role        string `json:"role"`       // "" | "owner" | "editor"
    Liked       bool   `json:"liked"`
}

type ListTrackersResponse struct {
    Trackers []TrackerResponse `json:"trackers"`
    Total    int               `json:"total"`
    Page     int               `json:"page"`
    PerPage  int               `json:"per_page"`
}

type ListSeriesResponse struct {
    Tracker TrackerResponse `json:"tracker"`
    Series  []SeriesModel   `json:"series"`
}

type ListValuesResponse struct {
    Series SeriesModel  `json:"series"`
    Values []ValueModel `json:"values"`
}
```

`PreviewResponse` は type 不問で同一フォーマット（`series[]` にマッピング）。coverage type の場合は entry 名が series 名、% が value になる。

### Context Key & アクセサ

```go
type contextKey int

const (
    trackerContextKey contextKey = iota
    seriesContextKey
    authCtxKey
    userIDCtxKey
)

func withTracker(ctx context.Context, tracker TrackerModel) context.Context {
    return context.WithValue(ctx, trackerContextKey, tracker)
}

func trackerFrom(ctx context.Context) (TrackerModel, bool) {
    m, ok := ctx.Value(trackerContextKey).(TrackerModel)
    return m, ok
}

func withSeries(ctx context.Context, series SeriesModel) context.Context {
    return context.WithValue(ctx, seriesContextKey, series)
}

func seriesFrom(ctx context.Context) (SeriesModel, bool) {
    s, ok := ctx.Value(seriesContextKey).(SeriesModel)
    return s, ok
}

func ContextWithAuth(ctx context.Context, userID *int64) context.Context {
    ctx = context.WithValue(ctx, authCtxKey, true)
    if userID != nil {
        ctx = context.WithValue(ctx, userIDCtxKey, *userID)
    }
    return ctx
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
    uid, ok := ctx.Value(userIDCtxKey).(int64)
    return uid, ok
}

func isAuthenticated(ctx context.Context) bool {
    v, ok := ctx.Value(authCtxKey).(bool)
    return ok && v
}
```

---

## HTTP ステータスコード

| Method | Path | Success | Error |
|--------|------|---------|-------|
| GET | `/api/tracker` | 200 | - |
| POST | `/api/tracker` | 201 | 400 |
| DELETE | `/api/tracker/{trackerId}` | 204 | 400, 403, 404 |
| PATCH | `/api/tracker/{trackerId}` | 200 | 400, 403, 404 |
| GET | `/api/tracker/{trackerId}/preview` | 200 | 400, 404 |
| GET | `/api/tracker/{trackerId}/series` | 200 | 400, 404 |
| POST | `/api/tracker/{trackerId}/series` | 201 | 400, 403, 404 |
| PATCH | `/api/tracker/{trackerId}/series/{seriesId}` | 200 | 400, 403, 404 |
| DELETE | `/api/tracker/{trackerId}/series/{seriesId}` | 204 | 400, 403, 404 |
| GET | `/api/tracker/{trackerId}/series/{seriesId}/values` | 200 | 400, 404 |
| POST | `/api/tracker/{trackerId}/series/{seriesId}/values` | 201 | 400, 403, 404 |
| DELETE | `/api/tracker/{trackerId}/series/{seriesId}/values` | 204 | 400, 403, 404 |
| POST | `/api/tracker/{trackerId}/like` | 201 | 400, 404 |
| DELETE | `/api/tracker/{trackerId}/like` | 204 | 400, 404 |

- POST は全エンドポイント **201 Created**
- DELETE は **204 No Content**
- 読み取り権限不足 → **403 Forbidden**（private tracker への匿名アクセス）

## エラーレスポンス

全エンドポイント共通（既存の `render` パッケージ経由）:

| 状況 | HTTP Status | body |
|------|-------------|------|
| リクエストボディ不正 (JSON decode failure) | 400 | `{"message":"invalid request body"}` |
| バリデーションエラー (重複や data_type 不正など) | 400 | `{"message":"<説明>"}` |
| URLパラメータ不正 (非数値ID) | 400 | `{"message":"invalid <field> id"}` |
| リソース未存在 | 404 | `{"message":"<resource> not found"}` |
| 認証エラー (API Key不一致) | 401 | `{"message":"invalid or missing token"}` |
| サーバ内部エラー | 500 | `{"message":"internal server error"}` |

---

## API エンドポイント詳細（swaggo コメント付き）

### Track

#### GET /api/tracker — トラッカー一覧 (ページネーション対応)

```go
// ListTrackers godoc
// @Summary      List trackers for current user
// @Description  Return trackers owned, edited, or liked by the current user
// @Tags         tracker
// @Success      200  {object}  tracker.ListTrackersResponse
// @Failure      401  {object}  core.ErrorResponse
// @Router       /api/tracker [get]
func (h *trackerHandler) listTrackers(w http.ResponseWriter, r *http.Request) {
```

ログイン中ユーザー: `userID` で `tracker_member` / `tracker_like` を JOIN した結果を返す（ページネーション対応）。
スーパーユーザー (userID=1): 全トラッカーを role="owner" で返す。
匿名ユーザー (userID=nil): 空配列を返す。

クエリパラメータ: `?page=N&per_page=N`（省略時は全件）

Response 200:
```json
{
    "trackers": [
        { "id": 1, "name": "build_metrics", "visibility": "private", "type": "tracker", "chart_config": "{}", "role": "owner", "liked": false },
        { "id": 2, "name": "performance",   "visibility": "public",  "type": "tracker", "chart_config": "{}", "role": "",     "liked": true  },
        { "id": 3, "name": "uptime",        "visibility": "unlisted","type": "tracker", "chart_config": "{}", "role": "editor","liked": false }
    ],
    "total": 3,
    "page": 1,
    "per_page": 12
}
```

#### POST /api/tracker — トラッカー作成

```go
// CreateTracker godoc
// @Summary      Create a tracker
// @Description  Add a new tracker. The name must be unique. Creator becomes owner.
//              type="coverage" requires repo_id to reference repository coverage data.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        body  body      tracker.CreateTrackerRequest  true  "Tracker information"
// @Success      201   {object}  tracker.TrackerModel
// @Failure      400   {object}  core.ErrorResponse
// @Failure      401   {object}  core.ErrorResponse
// @Router       /api/tracker [post]
func (h *trackerHandler) createTracker(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "name": "mora coverage", "visibility": "public", "type": "coverage", "repo_id": 1 }
```

or (type省略 = tracker):

```json
{ "name": "build_metrics", "visibility": "private" }
```

Response 201:
```json
{ "id": 1, "name": "mora coverage", "visibility": "public", "type": "coverage", "chart_config": "{}", "role": "owner", "liked": false }
```

- `type` 省略時は `"tracker"`（互換性維持）
- type=`"coverage"` で `repo_id` が無い → 400
- type=`"tracker"` で `repo_id` が非nil → 400
- type=`"coverage"` で常に series/values は空
- リクエストボディ 1MB 制限（`MaxBytesReader`）

#### DELETE /api/tracker/{trackerId} — トラッカー削除

```go
// DeleteTracker godoc
// @Summary      Delete a tracker
// @Description  Delete the specified tracker. Child series and values are also cascade-deleted.
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId} [delete]
func (h *trackerHandler) deleteTracker(w http.ResponseWriter, r *http.Request) {
```

- `requireEditPermission` ミドルウェアにより権限チェック
- 存在しない → 404

#### PATCH /api/tracker/{trackerId} — トラッカー更新

```go
// PatchTracker godoc
// @Summary      Update a tracker
// @Description  Update tracker fields (e.g. visibility). Requires edit permission.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                      true  "Tracker ID"
// @Param        body       body  tracker.PatchTrackerRequest  true  "Fields to update"
// @Success      200  {object}  tracker.TrackerResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId} [patch]
func (h *trackerHandler) patchTracker(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "visibility": "public" }
```

or:

```json
{ "chart_config": "{\"x_axis_label\":\"Date\",\"y_axis_label\":\"Count\",\"area\":true}" }
```

Response 200:
```json
{ "id": 1, "name": "build_metrics", "visibility": "public", "type": "tracker", "chart_config": "{\"x_axis_label\":\"Date\"}", "role": "owner", "liked": false }
```

- `visibility` と `chart_config` のいずれか1つ以上必須
- `visibility` は `"public"` / `"unlisted"` / `"private"` のみ許可
- `requireEditPermission` ミドルウェアにより権限チェック

#### GET /api/tracker/{trackerId}/preview — トラッカープレビュー

```go
// PreviewTracker godoc
// @Summary      Preview tracker data
// @Description  Return tracker info and preview data. For type="tracker": all series + latest 20 values per series.
//               For type="coverage": entry percentages mapped as series (total, go, ts, ...).
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      200  {object}  tracker.PreviewResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/preview [get]
func (h *trackerHandler) previewTracker(w http.ResponseWriter, r *http.Request) {
```

type=`tracker`:
各系列の最新20件の値を DESC で取得し ASC に反転して返す（従来通り）。

type=`coverage`:
`tracker_coverage.repo_id` から repository を特定し、`CoverageTimelineProvider.Timeline(repoID, 20)` で全 entry の過去20件の % を取得。entry 名 ("total", "go", "ts", ...) を series 名、% を value として `PreviewResponse.series[]` にマッピング（series.id=0, data_type="float"）。

Response 200 (coverage例):
```json
{
    "tracker": { "id": 5, "name": "mora", "type": "coverage", "chart_config": "{}", "role": "owner", "liked": false },
    "series": [
        { "series": { "id": 0, "tracker_id": 5, "name": "total", "data_type": "float", "config": "{}" },
          "values": [
            { "time": "2024-05-01T00:00:00Z", "value": 71.2 },
            { "time": "2024-06-01T00:00:00Z", "value": 76.5 }
          ] },
        { "series": { "id": 0, "tracker_id": 5, "name": "go", "data_type": "float", "config": "{}" },
          "values": [
            { "time": "2024-05-01T00:00:00Z", "value": 75.0 },
            { "time": "2024-06-01T00:00:00Z", "value": 80.0 }
          ] }
    ]
}
```

フロントエンドは type に依存せず同一フォーマットで描画可能。

---

### Like

#### POST /api/tracker/{trackerId}/like — いいね

```go
// LikeTracker godoc
// @Summary      Like a tracker
// @Description  Add a like to the specified tracker for the current user
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      201
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/like [post]
func (h *trackerHandler) likeTracker(w http.ResponseWriter, r *http.Request) {
```

- userID が nil → 403 Forbidden
- 重複いいねは 201 で正常終了（INSERT OR IGNORE）

#### DELETE /api/tracker/{trackerId}/like — いいね解除

```go
// UnlikeTracker godoc
// @Summary      Unlike a tracker
// @Description  Remove a like from the specified tracker for the current user
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/like [delete]
func (h *trackerHandler) unlikeTracker(w http.ResponseWriter, r *http.Request) {
```

- userID が nil → 403 Forbidden
- 存在しないいいねの解除は 204 で正常終了

---

### Series

#### GET /api/tracker/{trackerId}/series — シリーズ一覧

type=`tracker`: 指定されたトラッカーに属する全シリーズを返す（従来通り）。
type=`coverage`: 空配列 `[]` を返す。

```go
// ListSeries godoc
// @Summary      List all series
// @Description  Return all series belonging to the specified tracker. Returns empty for coverage-type trackers.
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Success      200  {object}  tracker.ListSeriesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/series [get]
func (h *trackerHandler) listSeries(w http.ResponseWriter, r *http.Request) {
```

Response 200 (tracker):
```json
{
    "tracker": { "id": 1, "name": "build_metrics", "visibility": "private", "type": "tracker", "chart_config": "{}", "role": "owner", "liked": false },
    "series": [
        { "id": 1, "tracker_id": 1, "name": "frontend_time", "data_type": "float", "config": "{}" },
        { "id": 2, "tracker_id": 1, "name": "build_count",   "data_type": "int", "config": "{}" }
    ]
}
```

#### POST /api/tracker/{trackerId}/series — シリーズ作成

```go
// CreateSeries godoc
// @Summary      Create a series
// @Description  Add a new series under a tracker. data_type must be "int" or "float" (defaults to "float").
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                       true  "Tracker ID"
// @Param        body       body  tracker.CreateSeriesRequest  true  "Series information"
// @Success      201  {object}  tracker.SeriesModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/series [post]
func (h *trackerHandler) createSeries(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "name": "frontend_time", "data_type": "float" }
```

Response 201:
```json
{ "id": 1, "tracker_id": 1, "name": "frontend_time", "data_type": "float", "config": "{}" }
```

- `data_type` 省略時はデフォルト `"float"`
- Handler 層で `"int"` / `"float"` 以外は 400 Bad Request
- `requireEditPermission` ミドルウェアにより権限チェック
- type=`coverage` のトラッカーに対しては 400 Bad Request（`"cannot modify series for coverage tracker"`）

#### PATCH /api/tracker/{trackerId}/series/{seriesId} — シリーズ更新

```go
// PatchSeries godoc
// @Summary      Update a series
// @Description  Update series fields (name, data_type, config). Requires edit permission.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                       true  "Tracker ID"
// @Param        seriesId   path  int                       true  "Series ID"
// @Param        body       body  tracker.PatchSeriesRequest  true  "Fields to update"
// @Success      200  {object}  tracker.SeriesModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/series/{seriesId} [patch]
func (h *trackerHandler) patchSeries(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "data_type": "int", "config": "{\"value_format\":\"%d\"}" }
```

Response 200:
```json
{ "id": 1, "tracker_id": 1, "name": "frontend_time", "data_type": "int", "config": "{\"value_format\":\"%d\"}" }
```

- `name`/`data_type`/`config` のいずれか1つ以上必須
- `data_type` は `"int"` / `"float"` のみ許可
- type=`coverage` のトラッカーに対しては 400 Bad Request

#### DELETE /api/tracker/{trackerId}/series/{seriesId} — シリーズ削除

```go
// DeleteSeries godoc
// @Summary      Delete a series
// @Description  Delete the specified series. Child values are also cascade-deleted.
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Param        seriesId   path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/series/{seriesId} [delete]
func (h *trackerHandler) deleteSeries(w http.ResponseWriter, r *http.Request) {
```

- `requireEditPermission` ミドルウェアにより権限チェック

### Values

#### GET /api/tracker/{trackerId}/series/{seriesId}/values — 値一覧

```go
// ListValues godoc
// @Summary      List all values
// @Description  Return time-series data for the specified series. The limit parameter restricts the maximum number of results.
// @Tags         tracker
// @Param        trackerId  path  int     true   "Tracker ID"
// @Param        seriesId   path  int     true   "Series ID"
// @Param        limit      query int     false  "Maximum number of results"
// @Success      200  {object}  tracker.ListValuesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/series/{seriesId}/values [get]
func (h *trackerHandler) listValues(w http.ResponseWriter, r *http.Request) {
```

Response 200:
```json
{
    "series": { "id": 1, "tracker_id": 1, "name": "frontend_time", "data_type": "float", "config": "{}" },
    "values": [
        { "time": "2024-01-01T00:00:00Z", "value": 45.0 },
        { "time": "2024-01-02T00:00:00Z", "value": 42.5 }
    ]
}
```

将来拡張案: `?after=<time>&limit=100`、`?from=...&to=...`

#### POST /api/tracker/{trackerId}/series/{seriesId}/values — 値追加

```go
// CreateValue godoc
// @Summary      Add a value
// @Description  Add time-series data to a series. Duplicate timestamps within the same series are not allowed.
// @Tags         tracker
// @Accept       json
// @Produce      json
// @Param        trackerId  path  int                     true  "Tracker ID"
// @Param        seriesId   path  int                     true  "Series ID"
// @Param        body       body  tracker.CreateValueRequest  true  "Value data"
// @Success      201  {object}  tracker.ValueModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/series/{seriesId}/values [post]
func (h *trackerHandler) createValue(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "time": "2024-01-01T00:00:00Z", "value": 45 }
```

Response 201:
```json
{ "id": 1, "series_id": 1, "time": "2024-01-01T00:00:00Z", "value": 45 }
```

- `series_id` は URL から自動設定（injectSeries ミドルウェアによる）
- 同一 series_id + time 重複 → 400
- `requireEditPermission` ミドルウェアにより権限チェック

#### DELETE /api/tracker/{trackerId}/series/{seriesId}/values — 値の全削除

```go
// DeleteValues godoc
// @Summary      Delete all values
// @Description  Delete all time-series data for the specified series
// @Tags         tracker
// @Param        trackerId  path  int  true  "Tracker ID"
// @Param        seriesId   path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      403  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/tracker/{trackerId}/series/{seriesId}/values [delete]
func (h *trackerHandler) deleteValues(w http.ResponseWriter, r *http.Request) {
```

- `requireEditPermission` ミドルウェアにより権限チェック

---

### Coverage type

coverage type のトラッカーは series/values を直接持たず、`tracker_coverage` テーブルで紐づいた repository の coverage データを表示する。

#### プレビュー (preview endpoint)

preview ハンドラは `tracker_coverage.repo_id` を取得し、`CoverageTimelineProvider.Timeline(repoID, 20)` を呼び出す。返ってきた coverage entry 名ごとの % を `PreviewResponse.series[]` にマッピングする:

| coverage entry | → Preview series name | data_type |
|---------------|----------------------|-----------|
| total | `"total"` | float |
| go | `"go"` | float |
| ts | `"ts"` | float |

#### series/values エンドポイントの動作

| Endpoint | type=`tracker` | type=`coverage` |
|----------|---------------|-----------------|
| GET /series | 一覧を返す | 空配列 `[]` を返す |
| POST /series | series 作成 | 400 "cannot modify..." |
| PATCH /series | series 更新 | 400 "cannot modify..." |
| DELETE /series | series 削除 | 400 "cannot modify..." |
| GET /values | 値を返す | —（series が空なのでアクセス不可）|
| POST /values | 値追加 | 400 "cannot modify..." |
| DELETE /values | 値削除 | 400 "cannot modify..." |

#### フロントエンド遷移

| 要素 | type=`tracker` | type=`coverage` |
|------|---------------|-----------------|
| カードクリック | `/tracker/{id}` | `/repos/{repo_id}/coverages` |
| detail view | 通常の series/values 管理 | チャート + リンクのみ |

detail view では、coverage tracker の詳細ページに「カバレッジ詳細を見る」リンクを表示し、クリックで coverage ページに遷移する。URL 直接アクセス時もエラーにはせず、情報ページとして ECharts チャートを表示する。

---

## 認証・認可

### 認証方式

認証は server パッケージの `requireTrackerAuth` middleware で一元管理:

1. **Session auth** — `SessionMiddleware` 経由。`MoraSession.IsLoggedIn()` で判定し、context に userID をセット
2. **API Key (Bearer)** — `Authorization: Bearer <token>` ヘッダで検証。`userStore.FindUserByAPIKey` でユーザーを特定
3. **匿名アクセス** — 上記いずれも成立しない場合、context に userID=nil をセット（読み取り専用）

### 判定フロー (server/server.go requireTrackerAuth)

```
1. Session auth あり？（MoraSession.IsLoggedIn()）
   → Yes: userID = sess.UserID() で context にセット
2. Bearer token あり？
   → Yes: userStore.FindUserByAPIKey(token) でユーザー検索
     - 見つかった → user.ID で context にセット
     - 見つからない → pass-through（次段へ）
3. → pass-through（匿名ユーザー、userID=nil）
```

401 は返さない pass-through middleware。認可判定は tracker パッケージ内の各ミドルウェアが行う。

### 3層ミドルウェア構成 (tracker/handler.go)

tracker handler 内では 3 つの認可ミドルウェアを使用:

```
requireAuth
  └── 常に通過。匿名ユーザーには userID=nil を context にセット
      └── requireReadPermission
          └── visibility に応じた読み取り制御
              └── requireEditPermission
                  └── 編集操作の前に権限チェック
```

#### requireAuth

```go
func (h *trackerHandler) requireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if isAuthenticated(r.Context()) {
            next.ServeHTTP(w, r)
            return
        }
        r = r.WithContext(ContextWithAuth(r.Context(), nil))
        next.ServeHTTP(w, r)
    })
}
```

- server 側の `requireTrackerAuth` がすでに context に auth フラグを設定済みの場合はそのまま通過
- 未認証（サーバー側で特に何もしなかった場合）は明示的に userID=nil をセット
- このミドルウェアは 401 を返さない。常に通過させる

#### requireReadPermission

```go
func (h *trackerHandler) requireReadPermission(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tracker, ok := trackerFrom(r.Context())
        if !ok {
            render.BadRequest(w, errors.New("no tracker in context"))
            return
        }
        if tracker.Visibility == "public" || tracker.Visibility == "unlisted" {
            next.ServeHTTP(w, r)
            return
        }
        uid, ok := UserIDFromContext(r.Context())
        if !ok {
            render.Forbidden(w, errors.New("this tracker is private"))
            return
        }
        if uid == 1 {
            next.ServeHTTP(w, r)
            return
        }
        member, _, err := h.store.isMember(uid, tracker.Id)
        if err != nil {
            render.InternalError(w, err)
            return
        }
        if !member {
            render.Forbidden(w, errors.New("this tracker is private"))
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

visibility による読み取り制御:

| visibility | 認証不要 | ログインユーザー | メンバー |
|------------|----------|-----------------|---------|
| public | 可 | 可 | 可 |
| unlisted | 可 | 可 | 可 |
| private | 不可 | 不可 | 可 |
| private (superuser) | — | — | 可 (userID=1) |

#### requireEditPermission

```go
func (h *trackerHandler) requireEditPermission(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        uid, ok := UserIDFromContext(r.Context())
        if !ok || uid == 0 {
            render.Forbidden(w, errors.New("anonymous users cannot edit"))
            return
        }
        if uid == 1 {
            next.ServeHTTP(w, r)
            return
        }
        tracker, ok := trackerFrom(r.Context())
        if !ok {
            render.BadRequest(w, errors.New("no tracker in context"))
            return
        }
        member, _, err := h.store.isMember(uid, tracker.Id)
        if err != nil {
            render.InternalError(w, err)
            return
        }
        if !member {
            render.Forbidden(w, errors.New("not a tracker member"))
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### ユーザー種別

| 種別 | userID | 読み取り権限 | 編集権限 |
|------|--------|-------------|---------|
| スーパーユーザー | 1 | 全トラッカー | 全トラッカー（API Key 認証時） |
| メンバー (owner) | 2+ | メンバーのトラッカー | メンバーのトラッカー |
| メンバー (editor) | 2+ | メンバーのトラッカー | メンバーのトラッカー（role 不問） |
| ログインユーザー (非メンバー) | 2+ | public/unlisted のみ | なし |
| 匿名ユーザー | nil | public/unlisted のみ | なし |

Like 系エンドポイント: userID==nil の場合 403。userID があれば誰でも実行可能。

### スーパーユーザー seed

`userStore.Init()` で `user` テーブルに id=1 のレコードを seed する:

```go
_, err := s.db.Exec(
    `INSERT OR IGNORE INTO user (id, username, avatar_url)
     VALUES (1, 'admin', '')`,
)
```

API key 認証時は `FindUserByAPIKey` で得た `user.ID` で識別する。API Key は `api_keys` テーブルで管理。

---

## ルーター構造 (handler.go)

```go
func newHandler(store *trackerStore) http.Handler {
    h := &trackerHandler{store: store}
    r := chi.NewRouter()

    r.Use(h.requireAuth)

    r.Route("/", func(r chi.Router) {
        r.Get("/", h.listTrackers)
        r.Post("/", h.createTracker)

        r.Route("/{trackerId}", func(r chi.Router) {
            r.Use(h.injectTracker)
            r.Use(h.requireReadPermission)
            r.With(h.requireEditPermission).Delete("/", h.deleteTracker)
            r.With(h.requireEditPermission).Patch("/", h.patchTracker)
            r.Post("/like", h.likeTracker)
            r.Delete("/like", h.unlikeTracker)
            r.Get("/preview", h.previewTracker)

            r.Route("/series", func(r chi.Router) {
                r.Get("/", h.listSeries)
                r.With(h.requireEditPermission).Post("/", h.createSeries)

                r.Route("/{seriesId}", func(r chi.Router) {
                    r.Use(h.injectSeries)
                    r.With(h.requireEditPermission).Patch("/", h.patchSeries)
                    r.With(h.requireEditPermission).Delete("/", h.deleteSeries)

                    r.Route("/values", func(r chi.Router) {
                        r.Get("/", h.listValues)
                        r.With(h.requireEditPermission).Post("/", h.createValue)
                        r.With(h.requireEditPermission).Delete("/", h.deleteValues)
                    })
                })
            })
        })
    })

    return r
}
```

### Server 側でのマウント

```go
trackerService, err := tracker.NewService(db)

s := &MoraServer{
    // ... existing fields ...
    tracker: trackerService,
}

// マウント（SessionMiddleware より後、requireTrackerAuth でラップ）
if s.tracker != nil {
    r.With(s.requireTrackerAuth).Mount("/api/tracker", s.tracker.Handler())
}
```

`r.With(s.requireTrackerAuth)` で session 認証 + API Key 認証を通ったユーザーに context フラグを付与。track handler 内の `requireAuth` がそのフラグを検知して通過させる（未認証時は userID=nil をセット）。

---

## ファイル詳細

### `tracker/store.go` — SQLite ストア

エラー変数:

```go
var (
    errorTrackerNotFound = errors.New("no tracker found")
    errorSeriesNotFound  = errors.New("no series found")
)
```

Store メソッド一覧:

```go
// Tracker
addTracker(tracker *TrackerModel, userID int64, repoID *int64) error
listTrackers(userID int64, page, perPage int) ([]TrackerResponse, int, error)
findTrackerById(id int64) (*TrackerModel, error)
findRepoIDByTrackerID(trackerID int64) (*int64, error)
deleteTracker(id int64) error
updateVisibility(id int64, visibility string) error
updateChartConfig(id int64, chartConfig string) error

// Series
addSeries(series *SeriesModel) error
findSeriesById(id int64) (*SeriesModel, error)
listSeries(trackerId int64) ([]SeriesModel, error)
updateSeries(series *SeriesModel) error
deleteSeries(id int64) error

// Value
addValue(value *ValueModel) error
listValues(seriesId int64, limit int) ([]ValueModel, error)
listLatestValues(seriesId int64, limit int) ([]ValueModel, error)
deleteValues(seriesId int64) error

// Member
isMember(userID, trackerID int64) (bool, string, error)

// Like
addLike(userID, trackerID int64) error
removeLike(userID, trackerID int64) error
isLiked(userID, trackerID int64) (bool, error)
```

### `tracker/provider.go` — CoverageTimelineProvider インターフェイス

```go
package tracker

import "time"

type CoverageTimelinePoint struct {
    Time  time.Time `json:"time"`
    Value float64   `json:"value"`
}

type CoverageTimelineProvider interface {
    Timeline(repoID int64, limit int) (map[string][]CoverageTimelinePoint, error)
}
```

- tracker パッケージは coverage パッケージに依存しない（循環依存回避）
- server パッケージが coverage store の実装を tracker service に注入する
- `Timeline` は repoID ごとに entry 名 ("total","go","ts",...) → []CoverageTimelinePoint のマップを返す
- limit: 各 entry の最新 N 件を取得

### `tracker/handler.go` — HTTP ハンドラ

- `trackerHandler` struct: `store *trackerStore`, `coverageProvider CoverageTimelineProvider`
- `newHandler(store *trackerStore, cp CoverageTimelineProvider) *trackerHandler`
- 3 層ミドルウェア: `requireAuth` → `requireReadPermission` → `requireEditPermission`
- `injectTracker` / `injectSeries` ミドルウェア
- CRUD ハンドラ 12個（PATCH /series 含む）+ swaggo コメント
- リクエスト型: `CreateTrackerRequest`, `CreateSeriesRequest`, `CreateValueRequest`, `PatchTrackerRequest`, `PatchSeriesRequest`（公開）
- レスポンス型: `ListTrackersResponse`, `PreviewResponse`, `ListSeriesResponse`, `ListValuesResponse`（公開）
- モデル: `TrackerModel`, `SeriesModel`, `ValueModel`（公開）
- Context Key とアクセサ: `ContextWithAuth` は exported, `isAuthenticated` は非公開
- GET values で `?limit=N` パース
- **coverage type**: preview で `CoverageTimelineProvider` からデータ取得、createTracker で type/repoID バリデーション、series/values ハンドラで coverage type を拒否
- **chart_config**: patchTracker で JSON 文字列として更新可能
- **series config**: patchSeries で JSON 文字列として更新可能（value_format 等）

### `server/session.go` — MoraSession.IsLoggedIn()

```go
func (s *MoraSession) IsLoggedIn() bool {
    s.lock.Lock()
    defer s.lock.Unlock()
    return len(s.tokenMap) > 0
}
```

- `tokenMap` が 1 つ以上の SCM token を持つ → logged in
- `requireTrackerAuth` middleware がこのメソッドを使って session 認証を判定

### `server/server.go` — requireTrackerAuth middleware + tracker mount

```go
func (s *MoraServer) requireTrackerAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, ok := MoraSessionFrom(r.Context())
        if ok && sess.IsLoggedIn() {
            r = r.WithContext(tracker.ContextWithAuth(r.Context(), sess.UserID()))
            next.ServeHTTP(w, r)
            return
        }
        token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        if token != "" && token != r.Header.Get("Authorization") {
            user, err := s.userStore.FindUserByAPIKey(token)
            if err == nil && user != nil {
                r = r.WithContext(tracker.ContextWithAuth(r.Context(), &user.ID))
                next.ServeHTTP(w, r)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

- pass-through middleware（認証失敗でも 401 は返さない。あくまで context にフラグをセットするだけ）
- Session auth 優先。Bearer token は session がない場合のみ検証
- `FindUserByAPIKey` は `api_keys` テーブルから検索（token 直値比較ではない）

### `tracker/service.go` — Service wrapper

```go
type Service struct {
    store            *trackerStore
    coverageProvider CoverageTimelineProvider
}

func NewService(db *sqlx.DB, cp CoverageTimelineProvider) (*Service, error)
func (s *Service) Handler() http.Handler

// Convenience methods for demo data seeding
func (s *Service) CreateTracker(name, visibility string, userID int64, trackerType string, repoID *int64) (*TrackerModel, error)
func (s *Service) CreateSeries(trackerID int64, name, dataType string) (*SeriesModel, error)
func (s *Service) CreateValue(seriesID int64, timestamp time.Time, value float64) (*ValueModel, error)
```

- `NewService` は `CoverageTimelineProvider` を受け取る（認証は server パッケージで完結）
- `CoverageTimelineProvider` は `tracker/provider.go` で定義されたインターフェイス
- Convenience メソッドは `server/demo.go` のシードデータ作成で使用

### `coverage/coverage_store.go` — Timeline メソッド追加 (v12)

coverage store が `tracker.CoverageTimelineProvider` インターフェイスを実装する:

```go
import "github.com/iszk1215/mora/tracker"

func (s *Store) Timeline(repoID int64, limit int) (map[string][]tracker.CoverageTimelinePoint, error)
```

- tracker パッケージに依存するが、tracker → coverage の依存はない（循環なし）
- 引数: repoID, limit（各 entry の最大件数）
- 戻り値: entry 名 ("total", "go", "ts", ...) をキー、時系列ポイント配列を値とするマップ
- 実装: `coverage_entry` テーブルから repo_id と LIMIT で検索、日時降順で取得後昇順に反転
- Go 側で entry 名ごとにグルーピングして返す

### `server/demo.go` — Seed データ追加 (v12)

既存の repository に対して coverage type の tracker を作成:

```go
for _, repo := range repos {
    s.tracker.CreateTracker(repo.Name+" coverage", "public", adminID, "coverage", &repo.ID)
}
```

### `server/server.go` — CoverageTimelineProvider の依存注入 (v12)

```go
trackerService, err := tracker.NewService(db, s.coverageStore)
```

- `s.coverageStore` は `*coverage.Store` 型だが、`CoverageTimelineProvider` インターフェイスを満たす

---

## テスト計画

テストファイル:
- `tracker/store_test.go` — Store 単体テスト（587行）
- `tracker/handler_test.go` — Handler 単体テスト（1203行）
- `tracker/service_test.go` — Service 単体テスト（109行）
- `frontend/src/tracker.test.tsx` — フロントエンドコンポーネントテスト（524行）
- `frontend/src/tracker_coverage.test.tsx` — Coverage tracker フロントエンドテスト（69行）

方法論:
- in-memory sqlite3 (`:memory:?_loc=auto`)
- Store: CRUD + UNIQUE/FK 制約違反 + ページネーション + listLatestValues + updateSeries/updateChartConfig
- Handler: 認証・認可（requireAuth/requireEditPermission/requireReadPermission）、正常系/異常系、visibility バリデーション、`?limit=` パース、preview エンドポイント、coverage type 拒否、PATCH /series
- Service: NewService、CreateTracker/CreateSeries/CreateValue
- Frontend: component render + fetch mock（MSW）

---

## 既存 UDM との差分サマリ

| 項目 | 既存 UDM | 新規 tracker |
|------|---------|------------|
| テーブル | `udm_metric`, `udm_item`, `udm_value` | `tracker`, `tracker_series`, `tracker_value` |
| repo_id 依存 | あり | なし（グローバル） |
| 認証 | server.apiKey（injectRepo） | Session + API Key 二重認証（server パッケージ） |
| 読み取り権限 | なし | visibility（public/unlisted/private） |
| 編集権限 | なし | tracker_member（owner/editor） |
| カスケード削除 | なし | あり（ON DELETE CASCADE） |
| 値の型 | TEXT | REAL（float64） |
| series の型 | `type` (int enum) | `data_type` (string) |
| POST レスポンス | 200/201 混在 | 全 POST で 201 |
| GET values | 全件 | `?limit=N` 対応 |
| ページネーション | なし | listTrackers page/per_page |
| OpenAPI | なし | swaggo コメント + exported types |
| CLI | あり | なし |
| Preview エンドポイント | なし | `GET /tracker/{id}/preview` |
| chart_config | なし | JSON（x/yラベル、area、legend、y_max） |
| series config | なし | JSON（value_format） |
| PATCH /series | なし | name/data_type/config 更新 |

---

## フロントエンド設計

### ルート構成

top-level (`/tracker`、repo 非依存、`/scms` と同列):

| Path | Page | 説明 | 編集権限 |
|------|------|------|---------|
| `/tracker` | TrackerView | カードグリッド一覧（ページネーション + プレビューチャート） | なし |
| `/tracker/new` | TrackerCreate | トラッカー作成フォーム（name + visibility + type/repoID） | 認証必須 |
| `/tracker/:trackerId` | TrackerDetailView | トラッカー詳細（type=tracker: 閲覧+Like+ECharts / type=coverage: 情報表示+coverage link） | なし |
| `/tracker/:trackerId/edit` | TrackerDetailEdit | トラッカー詳細編集（type=trackerのみ、type=coverageは非対応） | role 必須 |

`main.tsx` に `trackerRoute` として登録:

```typescript
export const trackerRoute = [
  { index: true, element: <TrackerView />, loader: loadTrackerList },
  { path: 'new', element: <TrackerCreate /> },
  {
    path: ':trackerId',
    loader: loadTrackerDetail,
    handle: { crumb: (p, d) => ({ label: d?.tracker?.name ?? 'Tracker' }) },
    children: [
      { index: true, element: <TrackerDetailView /> },
      {
        path: 'edit',
        element: <TrackerDetailEdit />,
        handle: { crumb: () => ({ label: 'Edit' }) },
      },
    ],
  },
]
```

### ファイル: `frontend/src/tracker.tsx`

#### 型定義（core.ts の TrackerResponse + tracker.tsx 内）

```typescript
// core.ts（共有）
interface ChartConfig { x_axis_label?: string; y_axis_label?: string; area?: boolean; show_legend?: boolean; y_max?: number }
interface SeriesConfig { value_format?: string }
interface SeriesModel { id: number; tracker_id: number; name: string; data_type: string; config: string }
interface TrackerResponse { id: number; name: string; visibility: string; type: string; repo_id?: number; chart_config: string; role: string; liked: boolean }

// tracker.tsx（内部）
interface ValueModel  { time: string; value: number }
interface SeriesValues { series: SeriesModel; values: ValueModel[] }
interface PaginatedTrackers {
  trackers: TrackerResponse[]
  total: number
  page: number
  per_page: number
}
interface PreviewData {
  tracker: TrackerResponse
  series: Array<{ series: SeriesModel; values: ValueModel[] }>
}
```

#### API 関数

| 関数 | Method | Path | 説明 |
|------|--------|------|------|
| `listTrackers(page?, perPage?)` | GET | `/api/tracker` | ページネーション対応一覧 |
| `fetchPreview(trackerId)` | GET | `/api/tracker/{id}/preview` | カードプレビュー（type=coverageも同一フォーマット） |
| `createTracker(name, visibility, type?, repoId?, chartConfig?)` | POST | `/api/tracker` | トラッカー作成 |
| `patchTracker(trackerId, visibility?, chartConfig?)` | PATCH | `/api/tracker/{id}` | visibility/chart_config 更新 |
| `deleteTracker` はフロントエンド未実装 | — | — | — |
| `listSeries(trackerId)` | GET | `/api/tracker/{id}/series` | 系列一覧（coverageは空配列） |
| `createSeries(trackerId, name, dataType)` | POST | `/api/tracker/{id}/series` | 系列作成（coverageは不可） |
| `patchSeries(trackerId, seriesId, name?, dataType?, config?)` | PATCH | `/api/tracker/{id}/series/{sid}` | 系列更新（coverageは不可） |
| `deleteSeries(trackerId, seriesId)` | DELETE | `/api/tracker/{id}/series/{sid}` | 系列削除（coverageは不可） |
| `createValue(trackerId, seriesId, time, value)` | POST | `/api/tracker/{id}/series/{sid}/values` | 値追加（coverageは不可） |
| `deleteValues(trackerId, seriesId)` | DELETE | `/api/tracker/{id}/series/{sid}/values` | 値全削除（coverageは不可） |
| `likeTracker(trackerId)` | POST | `/api/tracker/{id}/like` | いいね |
| `unlikeTracker(trackerId)` | DELETE | `/api/tracker/{id}/like` | いいね解除 |

#### TrackerView (`/tracker`)

トラッカーカードのグリッド表示 + ページネーション:

```
┌─ My Trackers ────────────────────────┐
│ ┌──────────┐  ┌──────────┐           │
│ │ Track A  │  │ Track B  │           │
│ │ (owner)  │  │ (editor) │           │
│ │ [charts] │  │ [charts] │           │
│ └──────────┘  └──────────┘           │
│ ┌─ Liked Tracks ───────────────────┐ │
│ │ ┌──────────┐  ┌──────────┐       │ │
│ │ │ Track C  │  │ Track D  │       │ │
│ │ │ [charts] │  │ [charts] │       │ │
│ │ └──────────┘  └──────────┘       │ │
│ └──────────────────────────────────┘ │
│ [Page 1 of 3]  < 1 2 3 >            │
└─────────────────────────────────────┘
```

- `role != ""` → "My Tracks"、`liked=true && role==""` → "Liked Tracks"
- 各カードに mini ECharts プレビュー表示（`GET /api/tracker/{id}/preview` を並列 fetch）
- カードクリック: type=`tracker` → `/tracker/{id}`、type=`coverage` → `/repos/{repo_id}/coverages`
- Loader: `loadTrackerList` = `listTrackers(1, 12)`

#### TrackerDetailView (`/tracker/:trackerId`)

- `TrackerDetailRouter` が `tracker.type` に応じてコンポーネントを分岐
  - type=`tracker` → `TrackerDetailView`
  - type=`coverage` → `CoverageTrackerDetail`（`tracker_coverage.tsx`）
- パンくず: Top > Tracker > `tracker.name`
- tracker 名の横に Like/Unlike ボタン、visibility バッジ
- type バッジ（"tracker" または "coverage"）
- Edit ボタン（`role != ""` かつ type=`tracker` の場合のみ表示）
- **type=`coverage`**: `CoverageTrackerDetail` が `/api/repos/{repo_id}/coverages` からデータ取得し `CoverageListContent` で表示。series/values 管理 UI は非表示。Edit ボタン非表示。
- **type=`tracker`**: `react-datepicker` による日付範囲フィルター + `TrackerChart`（ECharts）折れ線チャート。系列一覧テーブル（表示のみ）。
- Loader: `loadTrackerDetail` = `listSeries(trackerId)`

#### TrackerCreate (`/tracker/new`)

- トラッカー作成専用ページ
- Form:
  - name (text input, required)
  - visibility (select, options: public/unlisted/private)
  - type (select, options: tracker/coverage, default: tracker)
  - repo_id (number input, type=coverage 時のみ表示、必須)
- type 切替時に repo_id 欄の表示/非表示をトグル
- Create 成功 → `navigate(/tracker/:id)`（詳細画面へ）
- Create 失敗 → エラーメッセージ表示
- Cancel → `<Link to="/tracker">`
- アバタードロップダウンメニューにも「Create Tracker」リンク

#### TrackerDetailEdit (`/tracker/:trackerId/edit`)

- パンくず: Top > Tracker > `tracker.name` > Edit
- 系列作成・削除・更新、値追加・全削除（type=`coverage` の場合は非対応、アクセス時にメッセージ表示）
- visibility 変更セレクター
- chart_config 編集（x_axis_label, y_axis_label, area, show_legend, y_max）
- ECharts チャート（日付範囲フィルター付き）+ `TrackerChart` コンポーネント
- series config 編集（`ValueFormatCell` で value_format をインライン編集）
- `role == ""` でアクセス: Edit ボタンが表示されない（URL 直アクセス時はエラーにはならないが、coverage type の場合は Edit ボタンも表示しない）

### ファイル: `frontend/src/tracker_coverage.tsx`

- `CoverageTrackerDetail` コンポーネント: type=`coverage` のトラッカー専用ビュー
- `/api/repos/{repo_id}/coverages` からデータ取得し、既存の `CoverageListContent` で表示
- tracker チャートシステムを使わず、coverage モジュールの可視化に委譲

### ファイル: `frontend/src/chart.tsx`

- `TrackerChart`: ECharts ベースの汎用折れ線チャートコンポーネント
- data zoom、legend、ツールチップ、フォーマット付き表示
- `formatValue(value, config)`: printf 風フォーマット（`%d`, `%f`, `%.Nf` 対応）
- `Dataset` インターフェイス: `{ data, label, seriesConfig? }`

---

## 実装状態サマリ (current)

全ての v11, v12 機能が完了している。

### バックエンド

1. テーブル: `tracker`（visibility, type, chart_config）, `tracker_series`（data_type, config）, `tracker_value`, `tracker_member`, `tracker_like`, `tracker_coverage`
2. 全 CRUD エンドポイント（12個、PATCH /series 含む）
3. preview エンドポイント（tracker/coverage 両方対応）
4. ページネーション対応一覧
5. 3層ミドルウェア（requireAuth → requireReadPermission → requireEditPermission）
6. Session + API Key 二重認証（server パッケージで完結）
7. CoverageTimelineProvider インターフェイス + coverage store 実装
8. テスト: store_test.go（587行）, handler_test.go（1203行）, service_test.go（109行）

### フロントエンド

1. TrackerView（カードグリッド + ページネーション + プレビューチャート）
2. TrackerCreate（作成フォーム、type/repo_id 対応）
3. TrackerDetailRouter（type に応じたコンポーネント分岐）
4. TrackerDetailView（ECharts + 日付範囲フィルター + Like）
5. TrackerDetailEdit（系列/値のCRUD + visibility + chart_config + series config 編集）
6. CoverageTrackerDetail（coverage 専用ビュー、CoverageListContent 利用）
7. TrackerChart（ECharts ベースの汎用チャートコンポーネント）
8. テスト: tracker.test.tsx（524行）, tracker_coverage.test.tsx（69行）

### v12 (完了) — Coverage type 統合 + chart_config + PATCH /series

1. `tracker` テーブルに `type` カラム追加（DEFAULT 'tracker'）、`chart_config` カラム追加（DEFAULT '{}'）
2. `tracker_series` テーブルに `config` カラム追加（DEFAULT '{}'）
3. `tracker_coverage` テーブル追加（tracker_id → repo_id 紐づけ）
4. createTracker で type/RepoID バリデーション
5. POST/PATCH/DELETE series、POST/DELETE values で coverage type を 400 で拒否
6. GET series で coverage type は空配列を返す
7. preview で CoverageTimelineProvider からデータ取得
8. CoverageTimelineProvider インターフェイス定義（tracker/provider.go）
9. coverage store に Timeline(repoID, limit) メソッド追加
10. PATCH /series エンドポイント追加
11. patchTracker で chart_config 更新対応
12. server パッケージでの依存注入
13. demo.go の seed データ追加
14. フロントエンド: TrackerDetailRouter、CoverageTrackerDetail、TrackerChart、chart_config/series config 編集

---

## v10: ユーザー認証再設計

### 動機

- `user` テーブルに provider 情報が混在しており、1ユーザー複数IdP に対応できない
- `FindOrCreate` は login/signup の区別がなく、初回 OAuth で自動的にユーザーが作成されてしまう
- Gitea の SCM ユーザー取得が未対応（`createUserForSession` の `default: return`）
- `drone/go-scm` の `Users.Find()` はすべてのプロバイダで `scm.User.ID` が空のバグがある

### スキーマ

`user` テーブルから provider 情報を分離:

```sql
CREATE TABLE user (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username   TEXT NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE user_auth (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id          INTEGER NOT NULL REFERENCES user(id),
    provider         TEXT    NOT NULL,   -- "github" | "gitea"
    provider_user_id TEXT    NOT NULL,   -- プロバイダ側のユーザーID（文字列）
    UNIQUE(provider, provider_user_id)
);
```

superuser（API key 用）: `user (id=1, username='admin')` のみ。`user_auth` は作らない。

### データモデル比較 (v9 → v10)

| 項目 | v9 | v10 |
|------|----|-----|
| user テーブル | provider + provider_user_id 含む | provider 情報を user_auth に分離 |
| プロバイダ識別子 | `provider_user_id` (user テーブル) | `user_auth.provider_user_id` |
| 複数IdP | 不可 | 可能（後日管理画面対応） |
| ログイン動作 | `FindOrCreate` = なければ自動作成 | `FindByProvider` → なければ signup 画面へ |
| 初回サインアップ | なし（自動で作成） | 確認画面あり（/signup） |
| email 保存 | なし | なし（個人情報を極力保存しない方針） |
| SCM ライブラリ | drone/go-scm (v1.42.3) | 変更なし、drone/go-scm 継続 |
| ユーザー情報取得 | `fetchGitHubUser()` (raw API, GitHub のみ) | `fetchProviderUserInfo()` (raw API, GitHub + Gitea) |

### 方針

- `drone/go-scm` は継続使用（repo アクセス、OAuth2 トランスポートで問題なし）
- ユーザー情報取得のみ raw API を使う（`scm.Users.Find()` の ID バグを回避）
- email は保存しない（プロバイダ間の自動リンクに使うと危険、通知機能も今は不要）
- `FindOrCreate` を廃止し、`FindByProvider` + `CreateUser` / `LinkAuth` に分割

### UserStore インターフェース

```go
type UserStore interface {
    Init() error
    FindByProvider(provider, providerUserID string) (*User, error) // not found → sql.ErrNoRows
    CreateUser(username, avatarURL string) (*User, error)          // INSERT INTO user, return with id
    LinkAuth(userID int64, provider, providerUserID string) error  // INSERT INTO user_auth
    FindByID(id int64) (*User, error)                              // 既存
}
```

`FindOrCreate` は削除。

### Provider ユーザー情報取得

```go
type providerUserInfo struct {
    Provider       string // "github" | "gitea"
    ProviderUserID string // プロバイダ側のユーザーID
    Username       string
    AvatarURL      string
}

func fetchProviderUserInfo(rm RepositoryManager, token scm.Token) (*providerUserInfo, error)
```

Provider の判別は `rm.URL()` で行う:

- **GitHub**: `GET https://api.github.com/user` → `{"id": ..., "login": ..., "avatar_url": ...}`
  - 既存の `fetchGitHubUser` を統合・置き換え
- **Gitea**: `GET {scmURL}/api/v1/user` → `{"id": ..., "login": ..., "username": ..., "avatar_url": ...}`
  - ID は int、Login/Username の非空を優先
- Token は `Authorization: Bearer <token>` ヘッダで送信

### フロー

#### ログイン成功（既存ユーザー）

```
/login/{scm_id}
  → OAuth 認証
  → callback → token を sess.setToken(rmID, token)
  → fetchProviderUserInfo(rm, token) → {provider, providerUserID, ...}
  → userStore.FindByProvider(provider, providerUserID)

    - 見つかった → sess.SetUserID(user.ID), CSRFトークン発行, redirect /
    - 見つからない → sess.SetPendingSignup(info), redirect /signup
```

#### サインアップ（新規ユーザー）

```
GET /api/signup/pending
  → session から pendingSignup を返す（なければ 404）

POST /api/signup/confirm (CSRF required)
  → pendingSignup から CreateUser(username, avatarURL)
  → LinkAuth(userID, provider, providerUserID)
  → sess.SetUserID(user.ID), pendingSignup をクリア
  → redirect /scms
```

#### ログアウト

```
POST /logout (CSRF required)
  → 全 tokenMap 削除 + ClearUserID
  → redirect /scms
```

（v9 からの変更なし）

### Session 変更

```go
type pendingSignup struct {
    rmID           int64
    provider       string
    providerUserID string
    username       string
    avatarURL      string
}

type MoraSession struct {
    // ... 既存フィールド
    pendingSignup *pendingSignup   // nil の場合は pending 無し
}
```

pendingSignup アクセサ:

```go
func (s *MoraSession) SetPendingSignup(p *pendingSignup) { ... }
func (s *MoraSession) PendingSignup() *pendingSignup     { ... }
func (s *MoraSession) ClearPendingSignup()               { ... }
```

### ファイル変更一覧

| File | 変更内容 |
|------|---------|
| `server/user.go` | スキーマ変更、`FindOrCreate` → `FindByProvider`/`CreateUser`/`LinkAuth`、`fetchProviderUserInfo` 追加、`fetchGitHubUser`/`createUserForSession` 削除 |
| `server/user_test.go` | `FindOrCreate` → 新メソッドのテストに書き換え |
| `server/session.go` | `pendingSignup` フィールド + アクセサ追加 |
| `server/session_test.go` | 必要なら pendingSignup テスト追加 |
| `server/login.go` | `createLoginHandler` の user 処理（`createUserForSession`）→ `fetchProviderUserInfo` + `FindByProvider`/SetPendingSignup に差し替え、成功時 redirect `/` |
| `server/login_test.go` | 新しいフローに対応 |
| `server/signup.go` | **新規**: `GET /api/signup/pending` + `POST /api/signup/confirm` ハンドラ |
| `server/server.go` | signup ルートマウント、redirectHandler 調整（login → `/`, signup → `/scms`） |
| `server/server_test.go` | 必要に応じて signup テスト追加 |
| `frontend/src/main.tsx` | `/signup` ルート追加 |
| `frontend/src/signup.tsx` | **新規**: signup 確認ページコンポーネント |

### 補足

- DB マイグレーションは不要（既存 DB は開発用かつ破棄可能。`:memory:` で再作成される）
- `render` パッケージのエラーレスポンス型は流用
- CSRF トークンは既存の仕組み（`csrfCookieName` + `verifyCSRF`）を流用
- プロバイダの判別は `rm.URL()` の文字列に `"github.com"` が含まれるかで行う（既存と同様）

---

## v11: user_password テーブル分割

### 動機

- ほとんどのユーザーは IdP (SCM OAuth) 経由でログインし、パスワードを持たない
- `user` テーブルに `password_hash TEXT NULL` カラムがあるのは正規化として不適切
- `SELECT * FROM user` にパスワードハッシュが含まれるのがセキュリティ上好ましくない

### スキーマ

`user` テーブルから `password_hash` を分離し、新規 `user_password` テーブルに移動:

```sql
CREATE TABLE IF NOT EXISTS user_password (
    user_id       INTEGER PRIMARY KEY REFERENCES user(id) ON DELETE CASCADE,
    password_hash TEXT    NOT NULL,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);
```

- `PRIMARY KEY` = `user_id` により、1ユーザー1パスワード（1対1）
- `ON DELETE CASCADE` でユーザー削除時に自動削除
- パスワードを持たないユーザーは行が存在しない（`user` テーブルに `NULL` カラムを持たない）

### データモデル比較 (v10 → v11)

| 項目 | v10 | v11 |
|------|-----|-----|
| password 保存先 | `user.password_hash TEXT NULL` | `user_password.password_hash TEXT NOT NULL` |
| password 有無の判定 | `user.PasswordHash == nil` | `user_password` に行が存在するか |
| User 構造体 | `PasswordHash *string` フィールドあり | フィールドなし（完全に分離） |
| 認証時のクエリ | `SELECT * FROM user` で一度に取得 | `FindByUsername` + `GetPasswordHash(userID)` の2クエリ |

### UserStore インターフェース変更

```go
type UserStore interface {
    // ... 既存メソッド ...
    CreateUserWithPassword(username, password string) (*User, error) // 引数変更なし、内部で user_password に INSERT
    SetPassword(userID int64, passwordHash string) error             // UPDATE user → INSERT OR REPLACE INTO user_password
    GetPasswordHash(userID int64) (*string, error)                  // 新規: user_password から hash 取得
}
```

`User` 構造体から `PasswordHash` フィールドを削除:

```go
type User struct {
    ID        int64  `db:"id" json:"id"`
    Username  string `db:"username" json:"username"`
    AvatarURL string `db:"avatar_url" json:"avatar_url"`
    // PasswordHash は削除
    CreatedAt string `db:"created_at" json:"-"`
    UpdatedAt string `db:"updated_at" json:"-"`
}
```

### 認証フロー変更

パスワードログイン時:

```
FindByUsername(username)       // user 取得（PasswordHash なし）
  ↓
GetPasswordHash(user.ID)       // user_password から hash を取得
  ↓
hash == nil → "password not set" error
  ↓
bcrypt.CompareHashAndPassword([]byte(*hash), ...)
```

### マイグレーション

既存 DB からの移行 (`userStore.Init()` 内):

```go
// 1. 新テーブル作成
db.Exec(`CREATE TABLE IF NOT EXISTS user_password (...)`)
// 2. 既存データ移行
db.Exec(`INSERT OR IGNORE INTO user_password (user_id, password_hash)
         SELECT id, password_hash FROM user WHERE password_hash IS NOT NULL`)
// 3. 旧カラム削除 (3.35.0+). 新規インストール時はエラーになるが無視
_, _ = db.Exec(`ALTER TABLE user DROP COLUMN password_hash`)
```

従来の `ALTER TABLE user ADD COLUMN password_hash TEXT` 行は削除（v11 では不要）。

Admin seeding (`id=1 admin`) は:
1. `INSERT INTO user (id, username, avatar_url) VALUES (1, 'admin', '')`
2. `INSERT INTO user_password (user_id, password_hash) VALUES (1, ?)`（bcrypt hash of "admin"）

### フロントエンドへの影響

フロントエンドは `User` JSON に `password_hash` を含めておらず（`json:"-"`）、変更なし。

### ファイル変更一覧

| File | 変更内容 |
|------|---------|
| `server/user.go` | `User` から `PasswordHash` 削除、`Init()` に `user_password` テーブル作成 + 移行 + 旧カラム削除、`CreateUserWithPassword`/`SetPassword` を `user_password` に変更、`GetPasswordHash` 追加 |
| `server/user_test.go` | `PasswordHash` 参照 → `GetPasswordHash()` に変更 |
| `server/password.go` | login 処理: `user.PasswordHash` → `store.GetPasswordHash(user.ID)` に変更 |
| `server/password_test.go` | 同上 |
| `server/demo.go` | 変更なし（`CreateUserWithPassword` 経由） |
| フロントエンド | 変更なし |
