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

// UserCryptoAddress is a per-user TRON (TRC20) deposit address derived from the
// HD wallet at m/44'/195'/0'/0/{derivation_index}. The address and its signing
// key share that child key, so the wallet mnemonic can always re-derive the key
// to sweep funds out.
//
// 删除策略：硬删除（地址一经分配长期复用；删除仅用于彻底清理）。
type UserCryptoAddress struct {
	ent.Schema
}

func (UserCryptoAddress) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_crypto_addresses"},
	}
}

func (UserCryptoAddress) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("network").
			MaxLen(20).
			Default("TRC20"),
		field.String("address").
			MaxLen(64),
		// BIP44 派生序号；用于从助记词重新派生该地址的私钥。
		field.Int64("derivation_index"),
		// 链上余额缓存（USDT，6 位小数），避免每次查链。
		field.Float("last_balance").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,6)"}).
			Default(0),
		// ERC20 地址上的 USDC 余额缓存（6 位小数）。TRC20 行恒为 0。
		// USDT/USDC 共用同一 ERC20 地址，故同一行同时缓存两种代币余额。
		field.Float("last_balance_usdc").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,6)"}).
			Default(0),
		field.Time("last_balance_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
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

func (UserCryptoAddress) Indexes() []ent.Index {
	return []ent.Index{
		// 每个用户在每条链上只有一个地址。
		index.Fields("user_id", "network").Unique(),
		// 地址全局唯一。
		index.Fields("address").Unique(),
		// 派生序号在每条链上唯一（防止重复分配）。
		index.Fields("network", "derivation_index").Unique(),
	}
}
