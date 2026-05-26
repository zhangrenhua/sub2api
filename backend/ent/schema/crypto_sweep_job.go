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

// CryptoSweepJob represents one "one-click consolidation" run: it fans out into
// per-address CryptoSweepTask rows. The collection_address is snapshotted at
// job creation so a later config change cannot retroactively redirect an
// in-flight sweep.
//
// 删除策略：硬删除（运营记录，长期保留用于审计；删除仅用于清理）。
type CryptoSweepJob struct {
	ent.Schema
}

func (CryptoSweepJob) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "crypto_sweep_jobs"},
	}
}

func (CryptoSweepJob) Fields() []ent.Field {
	return []ent.Field{
		// pending / running / completed / failed
		field.String("status").
			MaxLen(20).
			Default("pending"),
		// 触发归集的管理员标识（如 "user:123"）。
		field.String("created_by").
			MaxLen(64).
			Default(""),
		field.Int("total_tasks").
			Default(0),
		field.Int("completed_tasks").
			Default(0),
		field.Float("total_swept").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,6)"}).
			Default(0),
		// 归集目标地址快照（创建任务时固定，防止改向）。
		field.String("collection_address").
			MaxLen(64),
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
		field.Time("finished_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (CryptoSweepJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("created_at"),
	}
}
