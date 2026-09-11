package main

import (
	"math"
	"sort"
	"time"
)

// Tx 一笔交易. Type 为 "deposit" 或 "withdraw". Amount 始终为正数.
type Tx struct {
	TS     time.Time
	Type   string
	Amount float64
}

// dailyFactor 把月利率(百分比, 30天)折算成等价日利率因子.
// 例如 monthlyRate=10 -> (1.1)^(1/30), 30天后余额恰好 ×1.1.
func dailyFactor(monthlyRate float64) float64 {
	if monthlyRate <= 0 {
		return 1
	}
	return math.Pow(1+monthlyRate/100.0, 1.0/30.0)
}

// balanceAt 计算在时刻 t 的余额(已含按天复利的利息).
// txs 不要求有序, 内部会按时间排序.
func balanceAt(txs []Tx, monthlyRate float64, t time.Time) float64 {
	if len(txs) == 0 {
		return 0
	}
	s := make([]Tx, len(txs))
	copy(s, txs)
	sort.Slice(s, func(i, j int) bool { return s[i].TS.Before(s[j].TS) })

	f := dailyFactor(monthlyRate)
	bal := 0.0
	var last time.Time
	have := false
	grow := func(to time.Time) {
		if !have || bal <= 0 || f == 1 {
			return
		}
		days := to.Sub(last).Hours() / 24.0
		if days > 0 {
			bal *= math.Pow(f, days)
		}
	}
	for _, tx := range s {
		if tx.TS.After(t) {
			break
		}
		grow(tx.TS)
		if tx.Type == "withdraw" {
			bal -= tx.Amount
		} else {
			bal += tx.Amount
		}
		if bal < 0 {
			bal = 0
		}
		last = tx.TS
		have = true
	}
	grow(t)
	return bal
}

// dailyPoint 日历上的一个点.
type dailyPoint struct {
	Date    string  `json:"date"`    // YYYY-MM-DD (本地)
	Balance float64 `json:"balance"` // 当天结束时的余额
}

// dailySeries 返回 [from, to] 之间每天结束时(当地 23:59:59)的余额序列.
func dailySeries(txs []Tx, monthlyRate float64, from, to time.Time, loc *time.Location) []dailyPoint {
	var out []dailyPoint
	d := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	for !d.After(end) {
		dayEnd := time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, loc)
		out = append(out, dailyPoint{
			Date:    d.Format("2006-01-02"),
			Balance: round2(balanceAt(txs, monthlyRate, dayEnd)),
		})
		d = d.AddDate(0, 0, 1)
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
