package config

import (
	"database/sql"
	"fmt"
)

// GetTraderStrategyStatusByStrategyID 获取指定策略的状态
func (d *Database) GetTraderStrategyStatusByStrategyID(traderID, strategyID string) (*TraderStrategyStatus, error) {
	query := `SELECT id, trader_id, strategy_id, symbol, had_position, status, entry_price, quantity, realized_pnl, updated_at 
		FROM trader_strategy_status 
		WHERE trader_id = ? AND strategy_id = ?`
	row := d.db.QueryRow(query, traderID, strategyID)

	var s TraderStrategyStatus
	var hadPos sql.NullBool
	err := row.Scan(&s.ID, &s.TraderID, &s.StrategyID, &s.Symbol, &hadPos, &s.Status, &s.EntryPrice, &s.Quantity, &s.RealizedPnL, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if hadPos.Valid {
		v := hadPos.Bool
		s.HadPosition = &v
	}
	return &s, nil
}

// GetLatestStrategyIDBySymbol 获取某个交易员 + 交易对 最新使用的策略ID
// 用于在手动平仓时，将对应策略标记为 CLOSED
func (d *Database) GetLatestStrategyIDBySymbol(traderID, symbol string) (string, error) {
	query := `
		SELECT strategy_id
		FROM strategy_decision_history
		WHERE trader_id = ? AND symbol = ?
		ORDER BY decision_time DESC
		LIMIT 1
	`

	var strategyID string
	err := d.db.QueryRow(query, traderID, symbol).Scan(&strategyID)
	if err != nil {
		return "", err
	}
	return strategyID, nil
}

// CloseStrategyForTrader 将某个策略标记为 CLOSED（仅影响指定交易员）
func (d *Database) CloseStrategyForTrader(traderID, strategyID string) error {
	query := `
		UPDATE trader_strategy_status
		SET status = 'CLOSED', updated_at = ` + d.getTimeFunc() + `
		WHERE trader_id = ? AND strategy_id = ?
	`
	_, err := d.db.Exec(query, traderID, strategyID)
	return err
}

// SaveParsedSignal 保存解析后的全局信号
func (d *Database) SaveParsedSignal(s *ParsedSignal) error {
	query := `INSERT INTO parsed_signals (signal_id, symbol, direction, received_at, content_json, raw_content)
              VALUES (?, ?, ?, ?, ?, ?)
              ON CONFLICT(signal_id) DO UPDATE SET
              content_json = excluded.content_json,
              raw_content = excluded.raw_content`

	if d.isMySQL {
		query = `INSERT INTO parsed_signals (signal_id, symbol, direction, received_at, content_json, raw_content)
                 VALUES (?, ?, ?, ?, ?, ?)
                 ON DUPLICATE KEY UPDATE 
                 content_json = VALUES(content_json),
                 raw_content = VALUES(raw_content)`
	}

	_, err := d.db.Exec(query, s.SignalID, s.Symbol, s.Direction, s.ReceivedAt, s.ContentJSON, s.RawContent)
	return err
}

// ParsedSignalExists 检查某个解析信号是否已存在（用于持久化去重）
func (d *Database) ParsedSignalExists(signalID string) (bool, error) {
	if signalID == "" {
		return false, nil
	}

	var one int
	err := d.db.QueryRow(`SELECT 1 FROM parsed_signals WHERE signal_id = ? LIMIT 1`, signalID).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

// GetAllParsedSignals 获取所有已解析的信号（按时间倒序）
func (d *Database) GetAllParsedSignals(limit int) ([]ParsedSignal, error) {
	query := `SELECT id, signal_id, symbol, direction, received_at, content_json, raw_content 
              FROM parsed_signals ORDER BY received_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ParsedSignal
	for rows.Next() {
		var s ParsedSignal
		err := rows.Scan(&s.ID, &s.SignalID, &s.Symbol, &s.Direction, &s.ReceivedAt, &s.ContentJSON, &s.RawContent)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}
