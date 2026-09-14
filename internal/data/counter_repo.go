package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

type CounterRepo struct {
	db      *sql.DB
	siteKey string

	mu sync.RWMutex

	// PV 计数（内存，定时持久化）
	sitePV int64
	pagePV map[string]int64

	// UV 去重（仅在内存中保存摘要，定时持久化到 SQLite）
	siteVisitors map[string]struct{}
	pageVisitors map[string]map[string]struct{} // page_key → set<visitor_hash>

	// 持久化定时器
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	stopErr  error
}

type CountResult struct {
	SitePV int64 `json:"site_pv"`
	PagePV int64 `json:"page_pv"`
	SiteUV int64 `json:"site_uv"`
	PageUV int64 `json:"page_uv"`
}

func NewCounterRepo(db *sql.DB, siteKey string) (*CounterRepo, error) {
	r := &CounterRepo{
		db:           db,
		siteKey:      siteKey,
		pagePV:       make(map[string]int64),
		siteVisitors: make(map[string]struct{}),
		pageVisitors: make(map[string]map[string]struct{}),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	// 从数据库恢复 PV 与 UV 计数
	if err := r.loadFromDB(); err != nil {
		return nil, err
	}

	// 启动定时持久化（每 30 秒）
	go r.periodicFlush(30 * time.Second)

	return r, nil
}

// loadFromDB 从数据库加载 PV 与 UV 计数到内存。
func (r *CounterRepo) loadFromDB() error {
	if err := r.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(value, 0) FROM site_stats WHERE key = ?`, r.siteKey,
	).Scan(&r.sitePV); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load site PV: %w", err)
	}

	rows, err := r.db.QueryContext(context.Background(),
		`SELECT page_key, page_count FROM page_stats WHERE site_key = ?`, r.siteKey)
	if err != nil {
		return fmt.Errorf("load page PV: %w", err)
	}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan page PV: %w", err)
		}
		r.pagePV[key] = count
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate page PV: %w", err)
	}
	_ = rows.Close()

	siteRows, err := r.db.QueryContext(context.Background(),
		`SELECT visitor_hash FROM site_visitors WHERE site_key = ?`, r.siteKey)
	if err != nil {
		return fmt.Errorf("load site visitors: %w", err)
	}
	for siteRows.Next() {
		var visitorHash string
		if err := siteRows.Scan(&visitorHash); err != nil {
			_ = siteRows.Close()
			return fmt.Errorf("scan site visitor: %w", err)
		}
		r.siteVisitors[visitorHash] = struct{}{}
	}
	if err := siteRows.Err(); err != nil {
		_ = siteRows.Close()
		return fmt.Errorf("iterate site visitors: %w", err)
	}
	_ = siteRows.Close()

	pageRows, err := r.db.QueryContext(context.Background(),
		`SELECT page_key, visitor_hash FROM page_visitors WHERE site_key = ?`, r.siteKey)
	if err != nil {
		return fmt.Errorf("load page visitors: %w", err)
	}
	for pageRows.Next() {
		var pageKey, visitorHash string
		if err := pageRows.Scan(&pageKey, &visitorHash); err != nil {
			_ = pageRows.Close()
			return fmt.Errorf("scan page visitor: %w", err)
		}
		if r.pageVisitors[pageKey] == nil {
			r.pageVisitors[pageKey] = make(map[string]struct{})
		}
		r.pageVisitors[pageKey][visitorHash] = struct{}{}
	}
	if err := pageRows.Err(); err != nil {
		_ = pageRows.Close()
		return fmt.Errorf("iterate page visitors: %w", err)
	}
	_ = pageRows.Close()
	return nil
}

// Increment 计数：PV++ 且 UV 去重
func (r *CounterRepo) Increment(ctx context.Context, pageKey, uid string) (*CountResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// PV
	r.sitePV++
	r.pagePV[pageKey]++

	// UV
	if uid != "" {
		visitorHash := hashVisitorID(uid)
		r.siteVisitors[visitorHash] = struct{}{}

		if r.pageVisitors[pageKey] == nil {
			r.pageVisitors[pageKey] = make(map[string]struct{})
		}
		r.pageVisitors[pageKey][visitorHash] = struct{}{}
	}

	return &CountResult{
		SitePV: r.sitePV,
		PagePV: r.pagePV[pageKey],
		SiteUV: int64(len(r.siteVisitors)),
		PageUV: int64(len(r.pageVisitors[pageKey])),
	}, nil
}

func hashVisitorID(uid string) string {
	digest := sha256.Sum256([]byte(uid))
	return hex.EncodeToString(digest[:])
}

// periodicFlush 定时将内存数据持久化到 SQLite
func (r *CounterRepo) periodicFlush(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = r.flush()
		case <-r.stopCh:
			r.stopErr = r.flush()
			close(r.doneCh)
			return
		}
	}
}

func (r *CounterRepo) flush() error {
	r.mu.RLock()
	sitePV := r.sitePV
	pagePV := make(map[string]int64, len(r.pagePV))
	for k, v := range r.pagePV {
		pagePV[k] = v
	}
	siteVisitors := make([]string, 0, len(r.siteVisitors))
	for visitorHash := range r.siteVisitors {
		siteVisitors = append(siteVisitors, visitorHash)
	}
	pageVisitors := make(map[string][]string, len(r.pageVisitors))
	for pageKey, visitors := range r.pageVisitors {
		for visitorHash := range visitors {
			pageVisitors[pageKey] = append(pageVisitors[pageKey], visitorHash)
		}
	}
	r.mu.RUnlock()

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin flush transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO site_stats (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?`,
		r.siteKey, sitePV, sitePV); err != nil {
		return fmt.Errorf("flush site PV: %w", err)
	}

	for key, count := range pagePV {
		if _, err := tx.Exec(`INSERT INTO page_stats (site_key, page_key, page_count) VALUES (?, ?, ?)
			ON CONFLICT(site_key, page_key) DO UPDATE SET page_count = ?`,
			r.siteKey, key, count, count); err != nil {
			return fmt.Errorf("flush page PV %q: %w", key, err)
		}
	}
	for _, visitorHash := range siteVisitors {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO site_visitors (site_key, visitor_hash) VALUES (?, ?)`,
			r.siteKey, visitorHash); err != nil {
			return fmt.Errorf("flush site visitor: %w", err)
		}
	}
	for pageKey, visitors := range pageVisitors {
		for _, visitorHash := range visitors {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO page_visitors (site_key, page_key, visitor_hash) VALUES (?, ?, ?)`,
				r.siteKey, pageKey, visitorHash); err != nil {
				return fmt.Errorf("flush page visitor %q: %w", pageKey, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit flush transaction: %w", err)
	}
	return nil
}

// Stop 停止定时刷新
func (r *CounterRepo) Stop() error {
	r.stopOnce.Do(func() { close(r.stopCh) })
	<-r.doneCh
	return r.stopErr
}
