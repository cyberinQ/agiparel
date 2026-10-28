// Gate 06.5-A: isolated, database-queued document extraction microservice.
// The worker does NOT receive public HTTP connections or production write grants.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyberinq/agiparel/services/document-intake/extract"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type job struct {
	tenant, id, filename string
	original             []byte
}

func claim(ctx context.Context, db *sql.DB) (job, bool, error) {
	var j job
	// Lock a job for no longer than the atomic claim; slow parsing occurs outside SQL transactions.
	err := db.QueryRowContext(ctx, `WITH picked AS (
    SELECT tenant_id,document_id FROM document_jobs_v65
    WHERE (status='QUEUED' OR (status='PROCESSING' AND updated_at<now()-interval '5 minutes'))
       AND attempts<3
    ORDER BY updated_at, document_id FOR UPDATE SKIP LOCKED LIMIT 1
  ) UPDATE document_jobs_v65 j SET status='PROCESSING',attempts=j.attempts+1,started_at=now(),updated_at=now(),outcome=''
  FROM picked p WHERE j.tenant_id=p.tenant_id AND j.document_id=p.document_id
  RETURNING j.tenant_id,j.document_id`).Scan(&j.tenant, &j.id)
	if errors.Is(err, sql.ErrNoRows) {
		return j, false, nil
	}
	if err != nil {
		return j, false, err
	}
	if err = db.QueryRowContext(ctx, `SELECT filename,original FROM document_sources_v65 WHERE tenant_id=$1 AND id=$2`, j.tenant, j.id).Scan(&j.filename, &j.original); err != nil {
		return j, false, err
	}
	return j, true, nil
}

func finish(ctx context.Context, db *sql.DB, j job, result extract.Result) error {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, s := range result.Segments {
		if _, e = tx.ExecContext(ctx, `INSERT INTO document_segments_v65(tenant_id,document_id,ordinal,kind,locator,content)
    VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, j.tenant, j.id, s.Ordinal, s.Kind, []byte(s.Locator), s.Content); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE document_jobs_v65
   SET status=$3,outcome=$4,segment_count=$5,finished_at=now(),updated_at=now()
   WHERE tenant_id=$1 AND document_id=$2`, j.tenant, j.id, result.Status, result.Explanation, len(result.Segments)); e != nil {
		return e
	}
	return tx.Commit()
}

func main() {
	url := os.Getenv("DOCUMENT_WORKER_DATABASE_URL")
	if url == "" {
		log.Fatal("DOCUMENT_WORKER_DATABASE_URL not configured")
	}
	db, e := sql.Open("pgx", url)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	log.Print("document-intake worker: ready (no external HTTP listener)")
	for ctx.Err() == nil {
		stepCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		j, ok, err := claim(stepCtx, db)
		cancel()
		if err != nil {
			log.Printf("document intake claim: %v", err)
		}
		if ok && err == nil {
			parseCtx, done := context.WithTimeout(ctx, 40*time.Second)
			res := extract.Process(parseCtx, j.filename, j.original)
			done()
			saveCtx, saveCancel := context.WithTimeout(ctx, 40*time.Second)
			if e = finish(saveCtx, db, j, res); e != nil {
				log.Printf("document %s finalize failed (will retry after stale lease): %v", j.id, e)
			} else {
				log.Printf("document %s status=%s segments=%d", j.id, res.Status, len(res.Segments))
			}
			saveCancel()
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}
