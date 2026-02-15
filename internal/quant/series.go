package quant

// NavPoint 单个净值数据点
type NavPoint struct {
	Date   string  // YYYY-MM-DD
	NAV    float64 // 单位净值
	AccNAV float64 // 累计净值（可选）
}

// NavSeries 基金净值时间序列（必须按日期升序排列）
type NavSeries struct {
	FundCode string
	Points   []NavPoint
}

// ReturnSeries 日收益率序列
type ReturnSeries struct {
	FundCode string
	Dates    []string  // 与 Returns 一一对应
	Returns  []float64 // 日收益率
}

// CurvePoint 时间序列数据点（供前端图表使用）
type CurvePoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// ToReturns 将净值序列转换为日收益率序列
func (s *NavSeries) ToReturns() *ReturnSeries {
	rs := &ReturnSeries{FundCode: s.FundCode}
	if len(s.Points) < 2 {
		return rs
	}
	rs.Dates = make([]string, 0, len(s.Points)-1)
	rs.Returns = make([]float64, 0, len(s.Points)-1)
	for i := 1; i < len(s.Points); i++ {
		prev := s.Points[i-1].NAV
		curr := s.Points[i].NAV
		if prev == 0 {
			continue
		}
		rs.Dates = append(rs.Dates, s.Points[i].Date)
		rs.Returns = append(rs.Returns, (curr-prev)/prev)
	}
	return rs
}
