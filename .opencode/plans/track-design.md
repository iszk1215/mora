# `/api/track` 設計書 (v7)

- **v7**: 二重認証（Session + API Key）設計 + フロントエンド設計を追加

## 概要

リポジトリに依存しない時系列データ追跡エンドポイント。既存の UDM (User Defined Metrics) と同様の概念だが、`repo_id` を持たずグローバルに利用できる。

- **パッケージ**: `track`（新規作成）
- **テーブル**: `track`, `track_series`, `track_value`（新規作成）
- **ファイル**: `track/store.go`, `track/handler.go`, `track/service.go`（計3ファイル）
- **既存ファイルの変更**: `server/server.go`（マウント + middleware 追加）, `server/session.go`（`IsLoggedIn()` 追加）
- **CLIは実装しない**（今回のスコープ外）
- **OpenAPI**: swaggo コメントを実装時から記述。レスポンス型は公開（exported）

---

## データモデル

### テーブル定義

```sql
CREATE TABLE IF NOT EXISTS track (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS track_series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id INTEGER NOT NULL REFERENCES track(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    data_type TEXT NOT NULL DEFAULT 'float',
    UNIQUE(track_id, name)
);

CREATE TABLE IF NOT EXISTS track_value (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES track_series(id) ON DELETE CASCADE,
    time DATETIME NOT NULL,
    value REAL NOT NULL,
    UNIQUE(series_id, time)
);
```

### 時刻精度について

`DATETIME` + Go の `time.Time` を使用する。

`go-sqlite3` ドライバは `time.Time` を ISO8601 文字列（例: `"2024-01-01T12:00:00.123456Z"`）として TEXT 保存する。このため SQLite の `DATETIME` 宣言でも**ナノ秒精度を正確に保持できる**。スキーマ変更不要。

### 階層構造

```
track (id, name)
  └── series (id, track_id, name, data_type)  — ON DELETE CASCADE
       └── value (id, series_id, time, value)  — ON DELETE CASCADE
```

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
type TrackModel struct {
    Id   int64  `json:"id"   db:"id"`
    Name string `json:"name" db:"name"`
}

type SeriesModel struct {
    Id       int64  `json:"id"        db:"id"`
    TrackId  int64  `json:"track_id"  db:"track_id"`
    Name     string `json:"name"      db:"name"`
    DataType string `json:"data_type" db:"data_type"`
}

type ValueModel struct {
    Id        int64     `db:"id"`
    SeriesId  int64     `db:"series_id"`
    Timestamp time.Time `json:"time"  db:"time"`
    Value     float64   `json:"value" db:"value"`
}
```

### レスポンス型（swaggo からの参照用に公開）

```go
// ListTracksResponse は GET /api/track のレスポンス
type ListTracksResponse struct {
    Tracks []TrackModel `json:"tracks"`
}

// ListSeriesResponse は GET /api/track/{trackId}/series のレスポンス
type ListSeriesResponse struct {
    Track  TrackModel    `json:"track"`
    Series []SeriesModel `json:"series"`
}

// ListValuesResponse は GET /api/track/{trackId}/series/{seriesId}/values のレスポンス
type ListValuesResponse struct {
    Series SeriesModel  `json:"series"`
    Values []ValueModel `json:"values"`
}
```

### Context Key & アクセサ

```go
type contextKey int

const (
    trackContextKey  contextKey = iota
    seriesContextKey contextKey = iota
    authCtxKey
)

func withTrack(ctx context.Context, track TrackModel) context.Context {
    return context.WithValue(ctx, trackContextKey, track)
}

func trackFrom(ctx context.Context) (TrackModel, bool) {
    m, ok := ctx.Value(trackContextKey).(TrackModel)
    return m, ok
}

func withSeries(ctx context.Context, series SeriesModel) context.Context {
    return context.WithValue(ctx, seriesContextKey, series)
}

func seriesFrom(ctx context.Context) (SeriesModel, bool) {
    s, ok := ctx.Value(seriesContextKey).(SeriesModel)
    return s, ok
}

// ContextWithAuth は server パッケージから呼ばれる exported 関数
func ContextWithAuth(ctx context.Context) context.Context {
    return context.WithValue(ctx, authCtxKey, true)
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
| GET | `/api/track` | 200 | - |
| POST | `/api/track` | 201 | 400 |
| DELETE | `/api/track/{trackId}` | 204 | 400, 404 |
| GET | `/api/track/{trackId}/series` | 200 | 400, 404 |
| POST | `/api/track/{trackId}/series` | 201 | 400, 404 |
| DELETE | `/api/track/{trackId}/series/{seriesId}` | 204 | 400, 404 |
| GET | `/api/track/{trackId}/series/{seriesId}/values` | 200 | 400, 404 |
| POST | `/api/track/{trackId}/series/{seriesId}/values` | 201 | 400, 404 |
| DELETE | `/api/track/{trackId}/series/{seriesId}/values` | 204 | 400, 404 |

- POST は全エンドポイント **201 Created**
- DELETE は **204 No Content**
- 認証エラーは全エンドポイント **401 Unauthorized**

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

#### GET /api/track — 全トラック一覧

```go
// ListTracks godoc
// @Summary      全トラックを取得
// @Description  登録されている全てのトラックを返す
// @Tags         track
// @Success      200  {object}  track.ListTracksResponse
// @Failure      401  {object}  core.ErrorResponse
// @Router       /api/track [get]
func (h *trackHandler) listTracks(w http.ResponseWriter, r *http.Request) {
```

Response 200:
```json
{
    "tracks": [
        { "id": 1, "name": "build_metrics" },
        { "id": 2, "name": "performance" }
    ]
}
```

#### POST /api/track — トラック作成

```go
// CreateTrack godoc
// @Summary      トラックを作成
// @Description  新しいトラックを追加する。name は一意である必要がある
// @Tags         track
// @Accept       json
// @Produce      json
// @Param        body  body      track.CreateTrackRequest  true  "トラック情報"
// @Success      201   {object}  track.TrackModel
// @Failure      400   {object}  core.ErrorResponse
// @Failure      401   {object}  core.ErrorResponse
// @Router       /api/track [post]
func (h *trackHandler) createTrack(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "name": "build_metrics" }
```

Response 201:
```json
{ "id": 1, "name": "build_metrics" }
```

CreateTrackRequest 型（swaggo 用に公開）:
```go
type CreateTrackRequest struct {
    Name string `json:"name"`
}
```

リクエストボディ 1MB 制限（`MaxBytesReader`）。

#### DELETE /api/track/{trackId} — トラック削除

```go
// DeleteTrack godoc
// @Summary      トラックを削除
// @Description  指定されたトラックを削除する。配下の series, value もカスケード削除される
// @Tags         track
// @Param        trackId  path  int  true  "Track ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId} [delete]
func (h *trackHandler) deleteTrack(w http.ResponseWriter, r *http.Request) {
```

- `trackId` 非数値 → 400, 存在しない → 404

---

### Series

#### GET /api/track/{trackId}/series — シリーズ一覧

```go
// ListSeries godoc
// @Summary      シリーズ一覧を取得
// @Description  指定されたトラックに属する全シリーズを返す
// @Tags         track
// @Param        trackId  path  int  true  "Track ID"
// @Success      200  {object}  track.ListSeriesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series [get]
func (h *trackHandler) listSeries(w http.ResponseWriter, r *http.Request) {
```

Response 200:
```json
{
    "track": { "id": 1, "name": "build_metrics" },
    "series": [
        { "id": 1, "track_id": 1, "name": "frontend_time", "data_type": "float" },
        { "id": 2, "track_id": 1, "name": "build_count",   "data_type": "int" }
    ]
}
```

#### POST /api/track/{trackId}/series — シリーズ作成

```go
// CreateSeries godoc
// @Summary      シリーズを作成
// @Description  トラック配下に新しいシリーズを追加する。data_type は "int" または "float"（省略時 "float"）
// @Tags         track
// @Accept       json
// @Produce      json
// @Param        trackId  path  int                     true  "Track ID"
// @Param        body     body  track.CreateSeriesRequest  true  "シリーズ情報"
// @Success      201  {object}  track.SeriesModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series [post]
func (h *trackHandler) createSeries(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "name": "frontend_time", "data_type": "float" }
```

Response 201:
```json
{ "id": 1, "track_id": 1, "name": "frontend_time", "data_type": "float" }
```

CreateSeriesRequest 型:
```go
type CreateSeriesRequest struct {
    Name     string `json:"name"`
    DataType string `json:"data_type"` // "int" または "float"
}
```

- `data_type` 省略時はデフォルト `"float"`
- Handler 層で `"int"` / `"float"` 以外は 400 Bad Request

#### DELETE /api/track/{trackId}/series/{seriesId} — シリーズ削除

```go
// DeleteSeries godoc
// @Summary      シリーズを削除
// @Description  指定されたシリーズを削除する。配下の value もカスケード削除される
// @Tags         track
// @Param        trackId   path  int  true  "Track ID"
// @Param        seriesId  path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId} [delete]
func (h *trackHandler) deleteSeries(w http.ResponseWriter, r *http.Request) {
```

---

### Values

#### GET /api/track/{trackId}/series/{seriesId}/values — 値一覧

```go
// ListValues godoc
// @Summary      値の一覧を取得
// @Description  指定されたシリーズの時系列データを返す。limit パラメータで最大件数を制限できる
// @Tags         track
// @Param        trackId   path  int     true   "Track ID"
// @Param        seriesId  path  int     true   "Series ID"
// @Param        limit     query int     false  "最大取得件数"
// @Success      200  {object}  track.ListValuesResponse
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId}/values [get]
func (h *trackHandler) listValues(w http.ResponseWriter, r *http.Request) {
```

Response 200:
```json
{
    "series": { "id": 1, "track_id": 1, "name": "frontend_time", "data_type": "float" },
    "values": [
        { "time": "2024-01-01T00:00:00Z", "value": 45.0 },
        { "time": "2024-01-02T00:00:00Z", "value": 42.5 }
    ]
}
```

将来拡張案: `?after=<time>&limit=100`、`?from=...&to=...`

#### POST /api/track/{trackId}/series/{seriesId}/values — 値追加

```go
// CreateValue godoc
// @Summary      値を追加
// @Description  シリーズに時系列データを追加する。同一系列・同一時刻のデータは重複不可
// @Tags         track
// @Accept       json
// @Produce      json
// @Param        trackId   path  int                   true  "Track ID"
// @Param        seriesId  path  int                   true  "Series ID"
// @Param        body      body  track.CreateValueRequest  true  "値データ"
// @Success      201  {object}  track.ValueModel
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId}/values [post]
func (h *trackHandler) createValue(w http.ResponseWriter, r *http.Request) {
```

Request:
```json
{ "time": "2024-01-01T00:00:00Z", "value": 45 }
```

Response 201:
```json
{ "id": 1, "series_id": 1, "time": "2024-01-01T00:00:00Z", "value": 45 }
```

CreateValueRequest 型:
```go
type CreateValueRequest struct {
    Timestamp time.Time `json:"time"`
    Value     float64   `json:"value"`
}
```

- `series_id` は URL から自動設定（JSON 内の series_id 不一致なら 400）
- 同一 series_id + time 重複 → 400

#### DELETE /api/track/{trackId}/series/{seriesId}/values — 値の全削除

```go
// DeleteValues godoc
// @Summary      値の全削除
// @Description  指定されたシリーズの全時系列データを削除する
// @Tags         track
// @Param        trackId   path  int  true  "Track ID"
// @Param        seriesId  path  int  true  "Series ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/series/{seriesId}/values [delete]
func (h *trackHandler) deleteValues(w http.ResponseWriter, r *http.Request) {
```

---

## 認証 (二重認証: Session + API Key)

`track` パッケージ内で 2 つの認証方式をサポート:

1. **Session auth** — 既存の `SessionMiddleware` 経由。server 側の `requireTrackAuth` middleware が session 確認後、context にフラグをセット
2. **API Key (Bearer)** — 従来通り。`Authorization: Bearer <token>` ヘッダで検証

### 判定フロー

```
1. Context に session auth フラグあり？
   → Yes: allow（SessionMiddleware 経由のログインユーザー）
2. API Key 未設定（h.apiKey == ""）？
   → Yes: allow（認証不要モード）
3. Bearer token が h.apiKey と一致？
   → Yes: allow（API Key 認証）
4. → 401 Unauthorized
```

### requireAuth (track/handler.go)

```go
func (h *trackHandler) requireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Session auth 済み
        if isAuthenticated(r.Context()) {
            next.ServeHTTP(w, r)
            return
        }
        // 2. API Key 未設定 → 認証不要
        if h.apiKey == "" {
            next.ServeHTTP(w, r)
            return
        }
        // 3. Bearer token 確認
        token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        if token == h.apiKey {
            next.ServeHTTP(w, r)
            return
        }
        render.Unauthorized(w, render.ErrInvalidToken)
    })
}
```

- `apiKey` は `NewService(db, apiKey)` → `Service` → `trackHandler` へ受け渡し（従来通り）
- `isAuthenticated()` は context の `authCtxKey` フラグを確認（server 側の `requireTrackAuth` がセット）
- 両方式は排他でなく、同時に使える

### requireTrackAuth (server/server.go)

session 認証を通ったユーザーに context フラグを付与する middleware。`SessionMiddleware` より後、track handler の前に配置する。

```go
func (s *MoraServer) requireTrackAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, ok := MoraSessionFrom(r.Context())
        if ok && sess.IsLoggedIn() {
            r = r.WithContext(track.ContextWithAuth(r.Context()))
        }
        next.ServeHTTP(w, r)
    })
}
```

### IsLoggedIn (server/session.go)

`MoraSession` に追加するメソッド。1 つ以上の SCM にログイン済みかを判定する。

```go
func (s *MoraSession) IsLoggedIn() bool {
    s.lock.Lock()
    defer s.lock.Unlock()
    return len(s.tokenMap) > 0
}
```

---

## ルーター構造 (handler.go)

```go
func newHandler(store *trackStore, apiKey string) http.Handler {
    h := &trackHandler{store: store, apiKey: apiKey}
    r := chi.NewRouter()

    r.Use(h.requireAuth)

    r.Route("/", func(r chi.Router) {
        r.Get("/", h.listTracks)
        r.Post("/", h.createTrack)

        r.Route("/{trackId}", func(r chi.Router) {
            r.Use(h.injectTrack)
            r.Delete("/", h.deleteTrack)

            r.Route("/series", func(r chi.Router) {
                r.Get("/", h.listSeries)
                r.Post("/", h.createSeries)

                r.Route("/{seriesId}", func(r chi.Router) {
                    r.Use(h.injectSeries)
                    r.Delete("/", h.deleteSeries)

                    r.Route("/values", func(r chi.Router) {
                        r.Get("/", h.listValues)
                        r.Post("/", h.createValue)
                        r.Delete("/", h.deleteValues)
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
track, err := track.NewService(db, os.Getenv("MORA_API_KEY"))

s := &MoraServer{
    // ... existing fields ...
    track: track,
}

// マウント（SessionMiddleware より後、requireTrackAuth でラップ）
if s.track != nil {
    r.With(s.requireTrackAuth).Mount("/api/track", s.track.Handler())
}
```

`r.With(s.requireTrackAuth)` で session 認証済みユーザーに context フラグを付与。track handler 内の `requireAuth` がそのフラグを検知して通過させる。

---

## ファイル詳細

### `track/store.go` — SQLite ストア

エラー変数:

```go
var (
    errorTrackNotFound   = errors.New("no track found")
    errorSeriesNotFound  = errors.New("no series found")
)
```

Store メソッド一覧:

```go
// Track
addTrack(track *TrackModel) error
listTracks() ([]TrackModel, error)
findTrackById(id int64) (*TrackModel, error)
deleteTrack(id int64) error

// Series
addSeries(series *SeriesModel) error
findSeriesById(id int64) (*SeriesModel, error)
listSeries(trackId int64) ([]SeriesModel, error)
deleteSeries(id int64) error

// Value
addValue(value *ValueModel) error
listValues(seriesId int64, limit int) ([]ValueModel, error)  // limit: 0 means no limit
deleteValues(seriesId int64) error
```

### `track/handler.go` — HTTP ハンドラ

- `trackHandler` struct: `store *trackStore`, `apiKey string`
- `requireAuth` ミドルウェア（Session auth フラグ + API Key Bearer の二重認証）
- `injectTrack` / `injectSeries` ミドルウェア
- CRUD ハンドラ 9個 + swaggo コメント
- リクエスト型: `CreateTrackRequest`, `CreateSeriesRequest`, `CreateValueRequest`（公開）
- レスポンス型: `ListTracksResponse`, `ListSeriesResponse`, `ListValuesResponse`（公開）
- モデル: `TrackModel`, `SeriesModel`, `ValueModel`（公開）
- Context Key とアクセサ: `authCtxKey` 追加（`ContextWithAuth` は exported, `isAuthenticated` は非公開）
- GET values で `?limit=N` パース

### `server/session.go` — MoraSession.IsLoggedIn()

```go
func (s *MoraSession) IsLoggedIn() bool {
    s.lock.Lock()
    defer s.lock.Unlock()
    return len(s.tokenMap) > 0
}
```

- `tokenMap` が 1 つ以上の SCM token を持つ → logged in
- `requireTrackAuth` middleware がこのメソッドを使って session 認証を判定

### `server/server.go` — requireTrackAuth middleware + track mount

```go
// SessionMiddleware より後、track handler の前に挿入
func (s *MoraServer) requireTrackAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, ok := MoraSessionFrom(r.Context())
        if ok && sess.IsLoggedIn() {
            r = r.WithContext(track.ContextWithAuth(r.Context()))
        }
        next.ServeHTTP(w, r)
    })
}
```

- pass-through middleware（認証失敗でも 401 は返さない。あくまで context にフラグをセットするだけ）
- 実際の認可判定は `track.requireAuth` が行う

### `track/service.go` — Service wrapper

```go
type Service struct {
    store  *trackStore
    apiKey string
}

func NewService(db *sqlx.DB, apiKey string) (*Service, error) {
    store := newTrackStore(db)
    err := store.initialize()
    if err != nil {
        return nil, fmt.Errorf("track store initialize: %w", err)
    }
    return &Service{store: store, apiKey: apiKey}, nil
}

func (s *Service) Handler() http.Handler {
    return newHandler(s.store, s.apiKey)
}
```

---

## テスト計画

テストファイル:
- `track/store_test.go` — Store 単体テスト
- `track/handler_test.go` — Handler 単体テスト
- `server/server_test.go` — 統合テスト（マウント確認）
- `frontend/src/track.test.tsx` — フロントエンドコンポーネントテスト

方法論:
- in-memory sqlite3 (`:memory:?_loc=auto`)
- Store: CRUD + UNIQUE/FK 制約違反
- Handler: requireAuth（Session auth + API Key 両方）、正常系/異常系、data_type バリデーション、`?limit=` パース
- Server: マウント確認、session auth 有無の挙動確認
- Frontend: component render + fetch mock（MSW）

---

## 既存 UDM との差分サマリ

| 項目 | 既存 UDM | 新規 track |
|------|---------|------------|
| テーブル | `udm_metric`, `udm_item`, `udm_value` | `track`, `track_series`, `track_value` |
| repo_id 依存 | あり | なし |
| API Key | server.apiKey（injectRepo） | Service.apiKey（requireAuth Bearer） |
| 認証 | injectRepo（server パッケージ） | Session + API Key 二重認証（track パッケージ内） |
| カスケード削除 | なし | あり（ON DELETE CASCADE） |
| 値の型 | TEXT | REAL（float64） |
| series の型 | `type` (int enum) | `data_type` (string) |
| POST レスポンス | 200/201 混在 | 全 POST で 201 |
| GET values | 全件 | `?limit=N` 対応 |
| OpenAPI | なし | swaggo コメント + exported types |
| CLI | あり | なし |

---

---

## フロントエンド設計

### ルート構成

top-level (`/track`、repo 非依存、`/scms` と同列):

| Path | Page | 説明 |
|------|------|------|
| `/track` | TrackList | 全トラック一覧、作成/削除 |
| `/track/:trackId` | TrackDetail | シリーズ一覧 + チャート + 値追加 |

`main.tsx` に `trackRoute` として登録（`udmRoute`/`coverageRoute` と同パターン）。

### ファイル: `frontend/src/track.tsx`

#### 型定義（バックエンド JSON と一致）

```typescript
interface TrackModel  { id: number; name: string }
interface SeriesModel { id: number; track_id: number; name: string; data_type: string }
interface ValueModel  { time: string; value: number }
```

#### API 関数（Udm と同様の fetch パターン）

| 関数 | Method | Path |
|------|--------|------|
| `listTracks()` | GET | `/api/track` |
| `createTrack(name)` | POST | `/api/track` |
| `deleteTrack(id)` | DELETE | `/api/track/{id}` |
| `listSeries(trackId)` | GET | `/api/track/{id}/series` |
| `createSeries(trackId, name, dataType?)` | POST | `/api/track/{id}/series` |
| `deleteSeries(trackId, seriesId)` | DELETE | `/api/track/{id}/series/{seriesId}` |
| `listValues(seriesId, limit?)` | GET | `/api/track/{id}/series/{id}/values?limit=N` |
| `createValue(seriesId, time, value)` | POST | `/api/track/{id}/series/{id}/values` |
| `deleteValues(trackId, seriesId)` | DELETE | `/api/track/{id}/series/{id}/values` |

#### 認証

Session cookie で自動認証。バックエンドの `SessionMiddleware` + `requireTrackAuth` + `requireAuth` がチェーンで処理。フロントエンド側で API key を扱う必要なし。

#### TrackList Page (`/track`)

- `<h1>Tracks</h1>` + インラインフォーム（name input + "Add Track" button）
- shadcn `<Table>` で一覧表示
- 各行: トラック名（`<Link to={/track/${id}}>`）+ 削除 `<Button>`
- Loader: `listTracks()`

#### TrackDetail Page (`/track/:trackId`)

- パンくず: Top > Track > `track.name`
- シリーズ一覧（table + create form）
- 全シリーズを重ねた ECharts 折れ線チャート 1枚（UdmChart と同パターン）
- 値追加フォーム（`<input type="datetime-local">` + value input + "Add" button）
- 日付範囲フィルター（DatePicker、Udm と同じ `react-datepicker`）
- Loader: `listSeries(trackId)` → `useEffect` で全 series の values を client-side fetch

---

## 実装順序

### バックエンド

1. `track/store.go` — テーブル作成 + CRUD
2. `track/store_test.go` — Store テスト
3. `track/handler.go` — ハンドラ + ルーター + ミドルウェア（二重認証） + exported types + swaggo コメント
4. `track/handler_test.go` — Handler テスト（Session auth + API Key）
5. `track/service.go` — Service ラッパー
6. `server/session.go` — `IsLoggedIn()` 追加
7. `server/server.go` — 変更（import, field, init, `requireTrackAuth`, mount）
8. `server/server_test.go` — 統合テスト追加
9. `docs/track-design.md` & `.opencode/plans/track-design.md` — v7 で更新
10. `go build ./...` でビルド確認
11. `make test` で全テスト実行

### フロントエンド

12. `frontend/src/track.tsx` — コンポーネント + loaders + API functions
13. `frontend/src/main.tsx` — `trackRoute` をインポートして登録
14. `frontend/src/track.test.tsx` — コンポーネントテスト
15. `make frontend-test` & `go build ./...` で確認
