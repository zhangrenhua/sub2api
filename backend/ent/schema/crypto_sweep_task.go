package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CryptoSweepTask is the per-address unit of a sweep job. It is a two-phase,
// idempotent, resumable state machine:
//
//	pending → gas_funding → gas_confirmed → sweeping → confirmed
//	                                                 ↘ failed
//
// Each tx hash is persisted BEFORE broadcast so a crash/restart re-checks the
// chain instead of re-sending.
//
// 删除策略：硬删除。
type CryptoSweepTask struct {
	ent.Schema
}

func (CryptoSweepTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "crypto_sweep_tasks"},
	}
}

func (CryptoSweepTask) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("job_id"),
		// Network this task targets (TRC20 / ERC20).
		field.String("network").
			MaxLen(20).
			Default("TRC20"),
		field.Int64("user_id").
			Default(0),
		field.String("address").
			MaxLen(64),
		field.Int64("derivation_index"),
		// 计划归集的 USDT 金额（6 位小数）。
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,6)"}),
		// pending / gas_funding / gas_confirmed / sweeping / confirmed / failed
		field.String("status").
			MaxLen(20).
			Default("pending"),
		// 阶段①：燃料地址给充值地址打 TRX 的交易哈希。
		field.String("gas_fund_tx").
			MaxLen(80).
			Default(""),
		// 阶段②：充值地址 → 归集地址的 USDT transfer 交易哈希。
		field.String("sweep_tx").
			MaxLen(80).
			Default(""),
		field.String("error").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (CryptoSweepTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("job_id"),
		index.Fields("status"),
	}
}
