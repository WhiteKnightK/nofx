package config

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// 当前数据库版本号
const CurrentSchemaVersion = 1

// Migration 迁移函数类型
type Migration func(*sql.DB) error

// migrations 所有迁移脚本，按版本号顺序
var migrations = map[int]Migration{
	1: migrationV1, // 添加 exchanges.provider 和 exchanges.label 字段
}

// migrationV1 迁移版本1：添加 exchanges.provider 和 exchanges.label 字段
func migrationV1(db *sql.DB) error {
	log.Println("🔄 开始执行数据库迁移 v1: 添加 exchanges.provider 和 exchanges.label 字段")

	// 检查字段是否已存在
	var providerExists, labelExists bool
	var err error

	// 检查 provider 字段
	err = db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		AND TABLE_NAME = 'exchanges' 
		AND COLUMN_NAME = 'provider'
	`).Scan(&providerExists)
	if err != nil {
		log.Printf("⚠️  检查 provider 字段失败: %v", err)
		providerExists = false
	}

	// 检查 label 字段
	err = db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		AND TABLE_NAME = 'exchanges' 
		AND COLUMN_NAME = 'label'
	`).Scan(&labelExists)
	if err != nil {
		log.Printf("⚠️  检查 label 字段失败: %v", err)
		labelExists = false
	}

	// 添加 provider 字段（如果不存在）
	if !providerExists {
		log.Println("  ➕ 添加 provider 字段...")
		_, err := db.Exec(`ALTER TABLE exchanges ADD COLUMN provider VARCHAR(100) DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("添加 provider 字段失败: %w", err)
		}
		log.Println("  ✅ provider 字段添加成功")
	} else {
		log.Println("  ✓ provider 字段已存在，跳过")
	}

	// 添加 label 字段（如果不存在）
	if !labelExists {
		log.Println("  ➕ 添加 label 字段...")
		_, err := db.Exec(`ALTER TABLE exchanges ADD COLUMN label VARCHAR(255) DEFAULT ''`)
		if err != nil {
			return fmt.Errorf("添加 label 字段失败: %w", err)
		}
		log.Println("  ✅ label 字段添加成功")
	} else {
		log.Println("  ✓ label 字段已存在，跳过")
	}

	// 数据迁移：如果 provider 为空，将其设置为 id（兼容旧数据）
	log.Println("  🔄 迁移旧数据：填充 provider 字段...")
	result, err := db.Exec(`
		UPDATE exchanges 
		SET provider = CASE 
			WHEN id LIKE 'binance%' THEN 'binance'
			WHEN id LIKE 'hyperliquid%' THEN 'hyperliquid'
			WHEN id LIKE 'aster%' THEN 'aster'
			WHEN id LIKE 'bitget%' THEN 'bitget'
			ELSE id
		END
		WHERE provider = '' OR provider IS NULL
	`)
	if err != nil {
		log.Printf("  ⚠️  填充 provider 字段失败（可能不需要）: %v", err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		log.Printf("  ✅ 已更新 %d 条记录的 provider 字段", rowsAffected)
	}

	// 数据迁移：如果 label 为空，将其设置为 name（兼容旧数据）
	log.Println("  🔄 迁移旧数据：填充 label 字段...")
	result, err = db.Exec(`
		UPDATE exchanges 
		SET label = name
		WHERE label = '' OR label IS NULL
	`)
	if err != nil {
		log.Printf("  ⚠️  填充 label 字段失败（可能不需要）: %v", err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		log.Printf("  ✅ 已更新 %d 条记录的 label 字段", rowsAffected)
	}

	log.Println("✅ 数据库迁移 v1 完成")
	return nil
}

// ensureSchemaVersionTable 确保 schema_version 表存在
func ensureSchemaVersionTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			description TEXT
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	return err
}

// getCurrentSchemaVersion 获取当前数据库版本
func getCurrentSchemaVersion(db *sql.DB) (int, error) {
	var version sql.NullInt64
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil // 没有版本记录，返回 0
	}
	return int(version.Int64), nil
}

// setSchemaVersion 设置数据库版本
func setSchemaVersion(db *sql.DB, version int, description string) error {
	_, err := db.Exec(`
		INSERT INTO schema_version (version, description) 
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE 
			applied_at = CURRENT_TIMESTAMP,
			description = ?
	`, version, description, description)
	return err
}

// RunMigrations 执行数据库迁移
func (d *Database) RunMigrations() error {
	if !d.isMySQL {
		// SQLite 不需要迁移系统，因为 ALTER TABLE 逻辑已经在 createTables 中处理
		return nil
	}

	log.Println("🔍 检查数据库迁移状态...")

	// 确保 schema_version 表存在
	if err := ensureSchemaVersionTable(d.db); err != nil {
		return fmt.Errorf("创建 schema_version 表失败: %w", err)
	}

	// 获取当前数据库版本
	currentVersion, err := getCurrentSchemaVersion(d.db)
	if err != nil {
		return fmt.Errorf("获取当前数据库版本失败: %w", err)
	}

	log.Printf("📊 当前数据库版本: %d, 目标版本: %d", currentVersion, CurrentSchemaVersion)

	// 如果版本已经是最新的，直接返回
	if currentVersion >= CurrentSchemaVersion {
		log.Println("✅ 数据库已是最新版本，无需迁移")
		return nil
	}

	// 执行需要执行的迁移
	for version := currentVersion + 1; version <= CurrentSchemaVersion; version++ {
		migration, exists := migrations[version]
		if !exists {
			log.Printf("⚠️  迁移版本 %d 不存在，跳过", version)
			continue
		}

		log.Printf("🚀 开始执行迁移 v%d...", version)
		if err := migration(d.db); err != nil {
			return fmt.Errorf("执行迁移 v%d 失败: %w", version, err)
		}

		// 记录迁移版本
		description := fmt.Sprintf("Migration v%d", version)
		if err := setSchemaVersion(d.db, version, description); err != nil {
			return fmt.Errorf("记录迁移版本失败: %w", err)
		}

		log.Printf("✅ 迁移 v%d 完成并已记录", version)
	}

	log.Println("✅ 所有数据库迁移完成")
	return nil
}

// 辅助函数：检查错误是否是"字段已存在"
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Duplicate column name") ||
		strings.Contains(errStr, "1060") ||
		strings.Contains(errStr, "duplicate")
}

