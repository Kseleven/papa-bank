package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB
var loc *time.Location

func main() {
	loc = time.Local // 容器时区由 TZ 环境变量决定, 默认 Asia/Shanghai

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://kids:kids@db:5432/kidsbank?sslmode=disable"
	}
	var err error
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("等待数据库... (%d) %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}
	if err := migrate(); err != nil {
		log.Fatalf("建表失败: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/accounts", listAccounts)
	mux.HandleFunc("POST /api/accounts", createAccount)
	mux.HandleFunc("DELETE /api/accounts/{id}", deleteAccount)
	mux.HandleFunc("PUT /api/accounts/{id}/rate", updateRate)
	mux.HandleFunc("GET /api/accounts/{id}/tx", listTx)
	mux.HandleFunc("POST /api/accounts/{id}/tx", createTx)
	mux.HandleFunc("PUT /api/tx/{txid}/note", updateNote)
	mux.HandleFunc("GET /api/accounts/{id}/daily", dailyHandler)
	mux.Handle("/", http.FileServer(http.Dir("./web")))

	addr := ":8080"
	log.Printf("小小存钱罐启动于 %s", addr)
	log.Fatal(http.ListenAndServe(addr, logMW(mux)))
}

func logMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}

func migrate() error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS accounts (
  id           SERIAL PRIMARY KEY,
  name         TEXT NOT NULL,
  monthly_rate DOUBLE PRECISION NOT NULL DEFAULT 10,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS transactions (
  id         SERIAL PRIMARY KEY,
  account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  type       TEXT NOT NULL CHECK (type IN ('deposit','withdraw')),
  amount     DOUBLE PRECISION NOT NULL CHECK (amount > 0),
  ts         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tx_acct ON transactions(account_id, ts);
-- 备注列: 对已有数据不影响, 旧记录 note 为空
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';
`)
	return err
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
func httpErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func loadTx(accountID int) ([]Tx, error) {
	rows, err := db.Query(`SELECT type, amount, ts FROM transactions WHERE account_id=$1 ORDER BY ts`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tx
	for rows.Next() {
		var t Tx
		if err := rows.Scan(&t.Type, &t.Amount, &t.TS); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type accountOut struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	MonthlyRate float64 `json:"monthly_rate"`
	Balance     float64 `json:"balance"`
}

func listAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, name, monthly_rate FROM accounts ORDER BY id`)
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []accountOut{}
	now := time.Now()
	for rows.Next() {
		var a accountOut
		if err := rows.Scan(&a.ID, &a.Name, &a.MonthlyRate); err != nil {
			httpErr(w, 500, err.Error())
			return
		}
		txs, _ := loadTx(a.ID)
		a.Balance = round2(balanceAt(txs, a.MonthlyRate, now))
		out = append(out, a)
	}
	writeJSON(w, out)
}

func createAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string  `json:"name"`
		MonthlyRate float64 `json:"monthly_rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		httpErr(w, 400, "需要 name")
		return
	}
	var id int
	err := db.QueryRow(`INSERT INTO accounts(name, monthly_rate) VALUES($1,$2) RETURNING id`,
		in.Name, in.MonthlyRate).Scan(&id)
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, accountOut{ID: id, Name: in.Name, MonthlyRate: in.MonthlyRate, Balance: 0})
}

func deleteAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	if _, err := db.Exec(`DELETE FROM accounts WHERE id=$1`, id); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func updateRate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var in struct {
		MonthlyRate float64 `json:"monthly_rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpErr(w, 400, "无效请求")
		return
	}
	if _, err := db.Exec(`UPDATE accounts SET monthly_rate=$1 WHERE id=$2`, in.MonthlyRate, id); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

type txOut struct {
	ID      int       `json:"id"`
	Type    string    `json:"type"`
	Amount  float64   `json:"amount"`
	Note    string    `json:"note"`
	TS      time.Time `json:"ts"`
	Balance float64   `json:"balance"`
}

func listTx(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var rate float64
	if err := db.QueryRow(`SELECT monthly_rate FROM accounts WHERE id=$1`, id).Scan(&rate); err != nil {
		httpErr(w, 404, "账户不存在")
		return
	}
	rows, err := db.Query(`SELECT id, type, amount, note, ts FROM transactions WHERE account_id=$1 ORDER BY ts`, id)
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var all []Tx
	var recs []txOut
	for rows.Next() {
		var o txOut
		if err := rows.Scan(&o.ID, &o.Type, &o.Amount, &o.Note, &o.TS); err != nil {
			httpErr(w, 500, err.Error())
			return
		}
		all = append(all, Tx{TS: o.TS, Type: o.Type, Amount: o.Amount})
		o.Balance = round2(balanceAt(all, rate, o.TS))
		recs = append(recs, o)
	}
	// 倒序返回(最新在前)
	for i, j := 0, len(recs)-1; i < j; i, j = i+1, j-1 {
		recs[i], recs[j] = recs[j], recs[i]
	}
	if recs == nil {
		recs = []txOut{}
	}
	writeJSON(w, recs)
}

func createTx(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var in struct {
		Type   string  `json:"type"`
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
		Date   string  `json:"date"` // 可选: YYYY-MM-DD, 补录当天记录
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpErr(w, 400, "无效请求")
		return
	}
	if in.Amount <= 0 || (in.Type != "deposit" && in.Type != "withdraw") {
		httpErr(w, 400, "金额需大于0, type 为 deposit/withdraw")
		return
	}
	var rate float64
	if err := db.QueryRow(`SELECT monthly_rate FROM accounts WHERE id=$1`, id).Scan(&rate); err != nil {
		httpErr(w, 404, "账户不存在")
		return
	}
	// 解析可选的补录日期. 若提供, 使用当天 12:00 本地时间;
	// 若是今天, 则用当前时间以便与其他今日记录顺序一致.
	ts := time.Now()
	if in.Date != "" {
		d, err := time.ParseInLocation("2006-01-02", in.Date, loc)
		if err != nil {
			httpErr(w, 400, "date 需为 YYYY-MM-DD")
			return
		}
		now := time.Now().In(loc)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		if d.Equal(today) {
			ts = now
		} else {
			ts = time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc)
		}
	}
	if in.Type == "withdraw" {
		txs, _ := loadTx(id)
		if balanceAt(txs, rate, ts) < in.Amount-1e-9 {
			httpErr(w, 400, "该时点余额不足")
			return
		}
	}
	if _, err := db.Exec(`INSERT INTO transactions(account_id, type, amount, note, ts) VALUES($1,$2,$3,$4,$5)`,
		id, in.Type, in.Amount, in.Note, ts); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func updateNote(w http.ResponseWriter, r *http.Request) {
	txid, _ := strconv.Atoi(r.PathValue("txid"))
	var in struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpErr(w, 400, "无效请求")
		return
	}
	res, err := db.Exec(`UPDATE transactions SET note=$1 WHERE id=$2`, in.Note, txid)
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpErr(w, 404, "记录不存在")
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func dailyHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var rate float64
	if err := db.QueryRow(`SELECT monthly_rate FROM accounts WHERE id=$1`, id).Scan(&rate); err != nil {
		httpErr(w, 404, "账户不存在")
		return
	}
	q := r.URL.Query()
	from, err1 := time.ParseInLocation("2006-01-02", q.Get("from"), loc)
	to, err2 := time.ParseInLocation("2006-01-02", q.Get("to"), loc)
	if err1 != nil || err2 != nil {
		httpErr(w, 400, "需要 from 和 to (YYYY-MM-DD)")
		return
	}
	txs, err := loadTx(id)
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, dailySeries(txs, rate, from, to, loc))
}
