// Author: CyberinQ LLC, 2026-10-26.
// DEV-ONLY, privileged bootstrap of material writer leases and enrolled CEO.
// The command never signs an assertion or grants CEO approval.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func required(label, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", label)
	}
	return nil
}

func requireDev() error {
	if os.Getenv("AGIPAREL_DEV_PROVISION") != "I_UNDERSTAND_FAKE_DATA" || os.Getenv("AGIPAREL_ENVIRONMENT") != "development" {
		return errors.New("refusing provisioning: this command is limited to explicit disposable development data")
	}
	return nil
}

func checkAdmin(ctx context.Context, db *sql.DB) error {
	var user string
	if err := db.QueryRowContext(ctx, `SELECT current_user`).Scan(&user); err != nil {
		return err
	}
	if user == "agiparel_app" {
		return errors.New("runtime agiparel_app role cannot provision authority")
	}
	// Requiring table write privileges also covers non-superuser privileged fixture roles.
	var canWrite bool
	if err := db.QueryRowContext(ctx, `SELECT has_table_privilege(current_user,'public.material_write_scopes_v5','INSERT') AND has_table_privilege(current_user,'public.material_write_scopes_v5','UPDATE') AND has_table_privilege(current_user,'public.material_ceo_principals_v5','INSERT') AND has_table_privilege(current_user,'public.material_ceo_principals_v5','UPDATE')`).Scan(&canWrite); err != nil {
		return err
	}
	if !canWrite {
		return errors.New("database role lacks authority-table provisioning privileges")
	}
	return nil
}

func provisionScope(ctx context.Context, db *sql.DB, tenant, order, plant, holder, approver string, duration time.Duration) error {
	for k, v := range map[string]string{"tenant": tenant, "order": order, "plant": plant, "holder": holder, "approved-by": approver} {
		if err := required(k, v); err != nil {
			return err
		}
	}
	if duration < time.Second || duration > 24*time.Hour {
		return errors.New("lease-duration must be >0 and <=24h")
	}
	_, err := db.ExecContext(ctx, `INSERT INTO public.material_write_scopes_v5
  (tenant_id,order_id,plant_id,mode,lease_holder,lease_revision,legacy_writer_fenced,approved_by,approved_at,lease_expires_at)
 VALUES ($1,$2,$3,'M3',$4,1,true,$5,now(),now()+($6::bigint * interval '1 second'))
 ON CONFLICT(tenant_id,order_id) DO UPDATE SET
  plant_id=EXCLUDED.plant_id, mode='M3', lease_holder=EXCLUDED.lease_holder,
  lease_revision=material_write_scopes_v5.lease_revision+1,
  legacy_writer_fenced=true, approved_by=EXCLUDED.approved_by,
  approved_at=now(),lease_expires_at=EXCLUDED.lease_expires_at`,
		tenant, order, plant, holder, approver, int64(duration/time.Second))
	return err
}

func enrollCEO(ctx context.Context, db *sql.DB, tenant, subject, enrolledBy string) error {
	for k, v := range map[string]string{"tenant": tenant, "subject": subject, "enrolled-by": enrolledBy} {
		if err := required(k, v); err != nil {
			return err
		}
	}
	_, err := db.ExecContext(ctx, `INSERT INTO public.material_ceo_principals_v5
  (tenant_id,subject,enrolled_by,active)
 VALUES($1,$2,$3,true)
 ON CONFLICT(tenant_id) DO UPDATE SET
  subject=EXCLUDED.subject,enrolled_by=EXCLUDED.enrolled_by,
  enrolled_at=now(),active=true`, tenant, subject, enrolledBy)
	return err
}

func run(args []string) error {
	if err := requireDev(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: materialctl scope|enroll-ceo [flags]")
	}
	url := os.Getenv("AGIPAREL_ADMIN_DATABASE_URL")
	if url == "" {
		return errors.New("AGIPAREL_ADMIN_DATABASE_URL is required")
	}
	command := args[0]
	if command != "scope" && command != "enroll-ceo" {
		return errors.New("expected scope or enroll-ceo")
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	tenant := fs.String("tenant", "", "tenant ID")
	approver := fs.String("approved-by", "", "external approver/auditor subject")
	var order, plant, holder *string
	var duration *time.Duration
	var subject *string
	if command == "scope" {
		order = fs.String("order", "", "order/PO ID")
		plant = fs.String("plant", "", "plant ID")
		holder = fs.String("holder", "", "configured server authority holder")
		duration = fs.Duration("lease-duration", time.Hour, "scope lease duration (max 24h)")
	} else {
		subject = fs.String("subject", "", "independently verified IdP CEO subject")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	// Validate all arguments BEFORE connecting and writing.
	if err := required("tenant", *tenant); err != nil {
		return err
	}
	if err := required("approved-by", *approver); err != nil {
		return err
	}
	if command == "scope" {
		for k, v := range map[string]string{"order": *order, "plant": *plant, "holder": *holder} {
			if err := required(k, v); err != nil {
				return err
			}
		}
		if *duration < time.Second || *duration > 24*time.Hour {
			return errors.New("lease duration must be >0 and <=24h")
		}
	} else if err := required("subject", *subject); err != nil {
		return err
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = checkAdmin(ctx, db); err != nil {
		return err
	}
	if command == "scope" {
		err = provisionScope(ctx, db, *tenant, *order, *plant, *holder, *approver, *duration)
	} else {
		err = enrollCEO(ctx, db, *tenant, *subject, *approver)
	}
	if err != nil {
		return err
	}
	fmt.Printf("Provisioned %s for tenant %s (development only)\n", command, *tenant)
	return nil
}
func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
