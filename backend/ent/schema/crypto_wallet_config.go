package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// CryptoWalletConfig holds the singleton configuration for the self-custodied
// TRON (TRC20) HD wallet: the encrypted master mnemonic, the monotonic
// derivation cursor used to assign per-user deposit addresses, and the
// collection (sweep destination) address.
//
// 删除策略：硬删除（单行配置，初始化后通常只更新不删除）。
// 安全：encrypted_mnemonic 由 WALLET_ENCRYPTION_KEY 加密，明文绝不入库。
type CryptoWalletConfig struct {
	ent.Schema
}

func (CryptoWalletConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "crypto_wallet_configs"},
	}
}

func (CryptoWalletConfig) Fields() []ent.Field {
	return []ent.Field{
		// 加密后的 BIP39 助记词（唯一备份载体；明文仅签名时在内存解密）。
		field.String("encrypted_mnemonic").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		// 下一个可分配给用户的派生序号。0 号保留给燃料(gas)钱包，用户地址从 1 开始。
		field.Int64("next_derivation_index").
			Default(1),
		// 归集目标地址（冷地址，服务器只存地址、不存其私钥）。
		field.String("collection_address").
			MaxLen(64).
			Default(""),
		// 燃料钱包地址（派生序号 0，持有 TRX 用于支付归集 gas）的快照，便于展示。
		field.String("fee_address").
			MaxLen(64).
			Default(""),
		// 是否已完成初始化（导入/生成助记词 + 设置归集地址）。
		field.Bool("initialized").
			Default(false),
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
