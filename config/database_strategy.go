package config

// GetTraderStrategyStatusByStrategyID 获取指定策略的状态
func (d *Database) GetTraderStrategyStatusByStrategyID(traderID, strategyID string) (*TraderStrategyStatus, error) {
	query := `SELECT id, trader_id, strategy_id, status, entry_price, quantity, realized_pnl, updated_at FROM trader_strategy_status WHERE trader_id = ? AND strategy_id = ?`
	row := d.db.QueryRow(query, traderID, strategyID)

	var s TraderStrategyStatus
	err := row.Scan(&s.ID, &s.TraderID, &s.StrategyID, &s.Status, &s.EntryPrice, &s.Quantity, &s.RealizedPnL, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
