# `/api/track` 設計書 (v8)

- **v7**: 二重認証（Session + API Key）設計 + フロントエンド設計を追加
- **v8**: ユーザー権限管理（track_member/track_like）、スーパーユーザー、フロントエンド閲覧/編集分離

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

-- v8: ユーザー権限管理
CREATE TABLE IF NOT EXISTS track_member (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    track_id INTEGER NOT NULL REFERENCES track(id) ON DELETE CASCADE,
    role     TEXT NOT NULL DEFAULT 'editor',   -- 'owner' | 'editor'
    PRIMARY KEY (user_id, track_id)
);

-- v8: ユーザーブックマーク（いいね）
CREATE TABLE IF NOT EXISTS track_like (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    track_id INTEGER NOT NULL REFERENCES track(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, track_id)
);
```

### 時刻精度について

`DATETIME` + Go の `time.Time` を使用する。

`go-sqlite3` ドライバは `time.Time` を ISO8601 文字列（例: `"2024-01-01T12:00:00.123456Z"`）として TEXT 保存する。このため SQLite の `DATETIME` 宣言でも**ナノ秒精度を正確に保持できる**。スキーマ変更不要。

### 階層構造

```
user
  ├── track_member (user_id, track_id, role)     — ON DELETE CASCADE
  ├── track_like   (user_id, track_id)           — ON DELETE CASCADE
  └── track (id, name)
        └── series (id, track_id, name, data_type)   — ON DELETE CASCADE
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
// v8: ユーザーごとの権限/いいね状態を含む
type TrackResponse struct {
    Id    int64  `json:"id"`
    Name  string `json:"name"`
    Role  string `json:"role"`   // "" | "owner" | "editor"（空文字=権限なし）
    Liked bool   `json:"liked"`
}

// ListTracksResponse は GET /api/track のレスポンス (v8: TrackModel→TrackResponse)
type ListTracksResponse struct {
    Tracks []TrackResponse `json:"tracks"`
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

const userIDCtxKey contextKey = iota

// ContextWithAuth は server パッケージから呼ばれる exported 関数
// v8: userID をオプションで受け取る（nil=匿名、1=superuser）
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
| GET | `/api/track` | 200 | - |
| POST | `/api/track` | 201 | 400 |
| DELETE | `/api/track/{trackId}` | 204 | 400, 403, 404 |
| GET | `/api/track/{trackId}/series` | 200 | 400, 404 |
| POST | `/api/track/{trackId}/series` | 201 | 400, 403, 404 |
| DELETE | `/api/track/{trackId}/series/{seriesId}` | 204 | 400, 403, 404 |
| GET | `/api/track/{trackId}/series/{seriesId}/values` | 200 | 400, 404 |
| POST | `/api/track/{trackId}/series/{seriesId}/values` | 201 | 400, 403, 404 |
| DELETE | `/api/track/{trackId}/series/{seriesId}/values` | 204 | 400, 403, 404 |
| POST | `/api/track/{trackId}/like` | 201 | 400, 404 |
| DELETE | `/api/track/{trackId}/like` | 204 | 400, 404 |

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

#### GET /api/track — トラック一覧 (v8: ユーザーごとにフィルタ)

```go
// ListTracks godoc
// @Summary      トラック一覧を取得
// @Description  現在のユーザーに関連するトラックを返す（所有/編集者/いいね済み）
// @Tags         track
// @Success      200  {object}  track.ListTracksResponse
// @Failure      401  {object}  core.ErrorResponse
// @Router       /api/track [get]
func (h *trackHandler) listTracks(w http.ResponseWriter, r *http.Request) {
```

ログイン中ユーザー: `userID` で `track_member` / `track_like` を JOIN した結果を返す。
スーパーユーザー (userID=1): 全トラックを role="owner" で返す。
匿名ユーザー (userID=nil): 空配列を返す。

Response 200:
```json
{
    "tracks": [
        { "id": 1, "name": "build_metrics", "role": "owner", "liked": false },
        { "id": 2, "name": "performance",   "role": "",     "liked": true  },
        { "id": 3, "name": "uptime",        "role": "editor","liked": false }
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
- **v8**: 権限チェック（track_member に含まれない && userID != 1 → 403）

---

### Like (v8)

#### POST /api/track/{trackId}/like — いいね

```go
// CreateLike godoc
// @Summary      トラックにいいね
// @Description  指定されたトラックに現在のユーザーのいいねを追加する
// @Tags         track
// @Param        trackId  path  int  true  "Track ID"
// @Success      201
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/like [post]
func (h *trackHandler) likeTrack(w http.ResponseWriter, r *http.Request) {
```

- userID が nil → 401
- 重複いいねは 201 で正常終了（INSERT OR IGNORE）

#### DELETE /api/track/{trackId}/like — いいね解除

```go
// DeleteLike godoc
// @Summary      いいねを解除
// @Description  指定されたトラックの現在のユーザーのいいねを削除する
// @Tags         track
// @Param        trackId  path  int  true  "Track ID"
// @Success      204
// @Failure      400  {object}  core.ErrorResponse
// @Failure      401  {object}  core.ErrorResponse
// @Failure      404  {object}  core.ErrorResponse
// @Router       /api/track/{trackId}/like [delete]
func (h *trackHandler) unlikeTrack(w http.ResponseWriter, r *http.Request) {
```

- userID が nil → 401
- 存在しないいいねの解除は 204 で正常終了

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

## 認証・認可 (v8)

### 認証方式

`track` パッケージ内で 2 つの認証方式をサポート:

1. **Session auth** — 既存の `SessionMiddleware` 経由。server 側の `requireTrackAuth` middleware が session 確認後、context に userID をセット
2. **API Key (Bearer)** — `Authorization: Bearer <token>` ヘッダで検証。API Key = スーパーユーザー (userID=1)

### 判定フロー

```
1. Context に session auth フラグあり？
   → Yes: allow（userID は session.UserID()）
2. API Key 未設定（h.apiKey == ""）？
   → Yes: allow（userID=nil、匿名アクセス、編集不可）
3. Bearer token が h.apiKey と一致？
   → Yes: allow（userID=1、スーパーユーザー、全権限）
4. → 401 Unauthorized
```

### requireAuth (track/handler.go)

```go
func (h *trackHandler) requireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if isAuthenticated(r.Context()) {
            next.ServeHTTP(w, r)
            return
        }
        if h.apiKey == "" {
            r = r.WithContext(ContextWithAuth(r.Context(), nil))
            next.ServeHTTP(w, r)
            return
        }
        token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        if token == h.apiKey {
            // API key = スーパーユーザー
            var uid int64 = 1
            r = r.WithContext(ContextWithAuth(r.Context(), &uid))
            next.ServeHTTP(w, r)
            return
        }
        render.Unauthorized(w, render.ErrInvalidToken)
    })
}
```

- 認証成功時に `ContextWithAuth(ctx, &userID)` を呼び、context に auth フラグ＋userID を設定
- スーパーユーザー (userID=1) は全トラックに対して全操作（編集・削除）が可能

### requireTrackAuth (server/server.go)

session 認証を通ったユーザーに context の userID を付与する middleware。

```go
func (s *MoraServer) requireTrackAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, ok := MoraSessionFrom(r.Context())
        if ok && sess.IsLoggedIn() {
            r = r.WithContext(track.ContextWithAuth(r.Context(), sess.UserID()))
        }
        next.ServeHTTP(w, r)
    })
}
```

### ユーザー種別

| 種別 | userID | 権限 |
|------|--------|------|
| スーパーユーザー | 1 | 全トラック編集・削除可能（API Key 認証時） |
| ログインユーザー | 2+ | 所有/編集者トラックのみ編集・削除可能 |
| 匿名ユーザー | nil | 参照のみ（API Key 未設定かつ未ログインの場合） |

### 編集権限チェック

編集系エンドポイント（POST/DELETE series, values, DELETE track）では以下のチェックを行う:

```
1. userID == 1 → allow（スーパーユーザー）
2. track_member に (userID, trackID) が存在 → allow（role 不問）
3. → 403 Forbidden
```

Like 系エンドポイントは userID==nil の場合 401。userID があれば誰でも実行可能。

### スーパーユーザー seed

`userStore.Init()` で `user` テーブルに id=1 のレコードを seed する:

```go
_, err := s.db.Exec(
    `INSERT OR IGNORE INTO user (id, provider, provider_user_id, username, avatar_url)
     VALUES (1, 'system', 'superuser', 'admin', '')`,
)
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
            r.Post("/like", h.likeTrack)       // v8
            r.Delete("/like", h.unlikeTrack)   // v8

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
// Track (v8: userID を引数に取るものあり)
addTrack(track *TrackModel, userID int64) error  // 作成者を track_member(role=owner) に追加
listTracks(userID int64) ([]TrackResponse, error)  // ユーザーの所有/いいね済みトラックを返す
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

// Member (v8)
isMember(userID, trackID int64) (bool, string, error)  // 存在確認 + role 取得

// Like (v8)
addLike(userID, trackID int64) error
removeLike(userID, trackID int64) error
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

## フロントエンド設計 (v8)

### ルート構成

top-level (`/track`、repo 非依存、`/scms` と同列):

| Path | Page | 説明 | 編集権限 |
|------|------|------|---------|
| `/track` | TrackListView | 「My Tracks」「Liked Tracks」セクション分け一覧 | なし |
| `/track/:trackId` | TrackDetailView | トラック詳細（閲覧のみ＋Likeボタン） | なし |
| `/track/:trackId/edit` | TrackDetailEdit | トラック詳細（編集用） | role 必須 |

`main.tsx` に `trackRoute` として登録:

```typescript
export const trackRoute = [
  { index: true, element: <TrackListView />, loader: loadTrackList },
  {
    path: ':trackId',
    loader: loadTrackDetail,
    handle: { crumb: (p, d) => ({ label: d?.track?.name ?? 'Track' }) },
    children: [
      { index: true, element: <TrackDetailView /> },
      {
        path: 'edit',
        element: <TrackDetailEdit />,
        handle: { crumb: () => ({ label: 'Edit' }) },
      },
    ],
  },
]
```

### ファイル: `frontend/src/track.tsx`

#### 型定義（バックエンド JSON と一致）

```typescript
// v8: role/liked 追加
interface TrackModel  { id: number; name: string; role: string; liked: boolean }
interface SeriesModel { id: number; track_id: number; name: string; data_type: string }
interface ValueModel  { time: string; value: number }
```

#### API 関数

| 関数 | Method | Path | v8変更 |
|------|--------|------|--------|
| `listTracks()` | GET | `/api/track` | 戻り値に role/liked 追加 |
| `createTrack(name)` | POST | `/api/track` | 変更なし |
| `deleteTrack(id)` | DELETE | `/api/track/{id}` | 変更なし |
| `likeTrack(trackId)` | POST | `/api/track/{id}/like` | **新規** |
| `unlikeTrack(trackId)` | DELETE | `/api/track/{id}/like` | **新規** |
| その他 series/value系 | — | — | 変更なし |

#### TrackListView (`/track`)

役割別にセクション分け:

```
┌─ My Tracks ────────────────────────┐
│  Track A (owner)          [Edit]   │
│  Track B (editor)         [Edit]   │
└────────────────────────────────────┘
┌─ Liked Tracks ─────────────────────┐
│  Track C                  [View]   │
│  Track D                  [View]   │
└────────────────────────────────────┘
```

- `role != ""` → "My Tracks"、`liked=true && role==""` → "Liked Tracks"
- `role != "" && liked=true` → "My Tracks" のみ（liked は冗長）
- Loader: `listTracks()`

#### TrackDetailView (`/track/:trackId`)

- パンくず: Top > Track > `track.name`
- track 名の横に Like/Unlike ボタン（user がいる場合のみ）
- 系列一覧テーブル（表示のみ、編集ボタンなし）
- 全シリーズを重ねた ECharts 折れ線チャート 1枚
- 日付範囲フィルター（DatePicker、`react-datepicker`）
- `role != ""` の場合: 「Edit」リンク表示 → `/track/:trackId/edit`
- Loader: `loadTrackDetail`（既存の `listSeries`）

#### TrackDetailEdit (`/track/:trackId/edit`)

- パンくず: Top > Track > `track.name` > Edit
- 現在の TrackDetail（系列作成・削除、値追加・削除）を移設
- `role == ""` でアクセス: リダイレクト or 404 表示

---

## 実装順序 (v8)

### バックエンド

1. `server/user.go` — `Init()` に superuser (id=1) seed を追加
2. `track/store.go` — `track_member`/`track_like` スキーマ追加、`TrackResponse` 型追加、既存メソッド変更 + 新規メソッド追加
3. `track/store_test.go` — 新規メソッドのテスト追加
4. `track/handler.go` — `ContextWithAuth` シグネチャ変更、`requireAuth` に userID 設定追加、編集権限チェック追加、`likeTrack`/`unlikeTrack` 追加、ルーター更新
5. `track/handler_test.go` — 権限チェック/Like のテスト追加
6. `server/server.go` — `requireTrackAuth` に userID 受け渡し追加
7. `server/server_test.go` — 統合テスト更新
8. `go build ./... && make test`

### フロントエンド

9. `frontend/src/track.tsx` — `TrackListView`/`TrackDetailView`/`TrackDetailEdit` に分割、like/unlike API 追加
10. `frontend/src/main.tsx` — ルート定義更新（view/edit 分離 + パンくず）
11. `frontend/src/track.test.tsx` — テスト更新
12. `make frontend-test && go build ./...`
