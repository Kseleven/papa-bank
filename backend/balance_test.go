package main

import (
	"math"
	"testing"
	"time"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestCompound(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	txs := []Tx{{TS: start, Type: "deposit", Amount: 100}}

	// 30天后应为 110
	got := balanceAt(txs, 10, start.AddDate(0, 0, 30))
	if !approx(got, 110) {
		t.Fatalf("30天后期望110, 得到 %.4f", got)
	}
	// 60天后复利滚动应为 121
	got = balanceAt(txs, 10, start.AddDate(0, 0, 60))
	if !approx(got, 121) {
		t.Fatalf("60天后期望121, 得到 %.4f", got)
	}
}

func TestMidDeposit(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	txs := []Tx{
		{TS: start, Type: "deposit", Amount: 100},
		{TS: start.AddDate(0, 0, 30), Type: "deposit", Amount: 100}, // 此时第一笔已变110
	}
	// 第30天结束: 110 + 100 = 210
	got := balanceAt(txs, 10, start.AddDate(0, 0, 30).Add(time.Second))
	if !approx(got, 210) {
		t.Fatalf("第30天期望210, 得到 %.4f", got)
	}
}

func TestWithdraw(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	txs := []Tx{
		{TS: start, Type: "deposit", Amount: 100},
		{TS: start.AddDate(0, 0, 30), Type: "withdraw", Amount: 50},
	}
	got := balanceAt(txs, 10, start.AddDate(0, 0, 30).Add(time.Second))
	if !approx(got, 60) { // 110 - 50
		t.Fatalf("取款后期望60, 得到 %.4f", got)
	}
}
