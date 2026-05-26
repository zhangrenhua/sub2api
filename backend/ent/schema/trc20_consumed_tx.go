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

// TRC20ConsumedTx is a ledger of on-chain USDT transfers that have already been
// credited to an order. The UNIQUE constraint on tx_hash is the guard that
// prevents a single deposit transaction from ever crediting two orders.
//
// 删除策略：硬删除（审计冗余记录在 payment_audit_logs；此表仅用于去重）。
type TRC20ConsumedTx struct {
	ent.Schema
}

func (TRC20ConsumedTx) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "trc20_consumed_txs"},
	}
}

func (TRC20ConsumedTx) Fields() []ent.Field {
	return []ent.Field{
		field.String("tx_hash").
			MaxLen(80),
		field.Int64("order_id"),
		field.String("address").
			MaxLen(64),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,6)"}),
		field.Time("confirmed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TRC20ConsumedTx) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tx_hash").Unique(),
		index.Fields("order_id"),
	}
}
