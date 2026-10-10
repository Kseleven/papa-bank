// 与日历一致，按浏览器本地日期统计；本周从周一开始。
// anchor 是用户选择的时间点，now 用于限制当前时间之后的记录。
function summaryPeriod(period, now = new Date(), anchor = now) {
  const ref = new Date(anchor);
  if (!Number.isFinite(ref.getTime())) return summaryPeriod(period, now, now);
  let start, end;
  if (period === 'week') {
    start = new Date(ref.getFullYear(), ref.getMonth(), ref.getDate());
    start.setDate(start.getDate() - (start.getDay() + 6) % 7);
    end = new Date(start.getFullYear(), start.getMonth(), start.getDate() + 6, 23, 59, 59, 999);
  } else {
    start = new Date(ref.getFullYear(), ref.getMonth(), 1);
    end = new Date(ref.getFullYear(), ref.getMonth() + 1, 0, 23, 59, 59, 999);
  }
  if (end > now) end = new Date(now);
  return {start, end};
}

// 月份切换固定在 1 日，避免 31 日向短月份切换时跳过一个月。
function shiftSummaryDate(period, anchor, direction) {
  if (period === 'month') return new Date(anchor.getFullYear(), anchor.getMonth() + direction, 1);
  const {start} = summaryPeriod('week', anchor, anchor);
  return new Date(start.getFullYear(), start.getMonth(), start.getDate() + direction * 7);
}

function opportunityGrowth(amount, monthlyRate, asOf = new Date()) {
  const daily = monthlyRate > 0 ? Math.pow(1 + monthlyRate / 100, 1 / 30) : 1;
  const horizons = [
    {key: 'week', label: '未来一周', days: 7},
    {key: 'month', label: '未来一个月', days: 30},
    {key: 'year', label: '未来一年', days: 365}
  ];
  return horizons.map(horizon => {
    const total = amount * Math.pow(daily, horizon.days);
    return { ...horizon, total: Math.round((total + Number.EPSILON) * 100) / 100,
      gain: Math.round((total - amount + Number.EPSILON) * 100) / 100 };
  });
}

function opportunityBase(records, monthlyRate, asOf) {
  const daily = monthlyRate > 0 ? Math.pow(1 + monthlyRate / 100, 1 / 30) : 1;
  return records.reduce((total, record) => {
    if (record.type !== 'withdraw') return total;
    const ts = new Date(record.ts);
    const amount = Number(record.amount);
    if (!Number.isFinite(ts.getTime()) || !Number.isFinite(amount) || amount <= 0 || ts > asOf) return total;
    const days = Math.max(0, (asOf.getTime() - ts.getTime()) / 86400000);
    return total + amount * Math.pow(daily, days);
  }, 0);
}

function buildSummary(records, period, now = new Date(), anchor = now) {
  const {start, end} = summaryPeriod(period, now, anchor);
  const depositGroups = new Map();
  const withdrawalGroups = new Map();
  const days = new Set();
  let depositCount = 0, depositAmount = 0, withdrawalCount = 0, withdrawalAmount = 0;
  const periodWithdrawals = [];
  for (const record of records) {
    const ts = new Date(record.ts);
    if ((record.type !== 'deposit' && record.type !== 'withdraw') || ts < start || ts > end || !Number.isFinite(ts.getTime())) continue;
    const value = Number(record.amount);
    if (!Number.isFinite(value) || value <= 0) continue;
    const note = (record.note || '').trim();
    const groups = record.type === 'deposit' ? depositGroups : withdrawalGroups;
    const label = record.type === 'deposit' ? '存入' : '取出';
    const group = groups.get(note) || {name: note || `未填写备注的${label}`, unnamed: !note, count: 0, amount: 0};
    group.count++;
    group.amount += value;
    groups.set(note, group);
    if (record.type === 'deposit') {
      depositCount++;
      depositAmount += value;
    } else {
      withdrawalCount++;
      withdrawalAmount += value;
      periodWithdrawals.push(record);
    }
    days.add(`${ts.getFullYear()}-${ts.getMonth()}-${ts.getDate()}`);
  }
  const round = value => Math.round((value + Number.EPSILON) * 100) / 100;
  const sortGroups = groups => [...groups.values()].map(group => ({...group, amount: round(group.amount)}))
    .sort((a, b) => Number(a.unnamed) - Number(b.unnamed) || b.count - a.count || b.amount - a.amount || a.name.localeCompare(b.name, 'zh-CN'));
  return {
    start, end, days: days.size,
    depositCount, depositAmount: round(depositAmount),
    withdrawalCount, withdrawalAmount: round(withdrawalAmount),
    withdrawalOpportunityRecords: periodWithdrawals,
    netAmount: round(depositAmount - withdrawalAmount),
    // 保留旧字段，便于页面外部调用方平滑升级。
    count: depositCount, amount: round(depositAmount),
    groups: sortGroups(depositGroups),
    depositGroups: sortGroups(depositGroups),
    withdrawalGroups: sortGroups(withdrawalGroups)
  };
}

if (typeof module !== 'undefined') module.exports = {summaryPeriod, shiftSummaryDate, opportunityGrowth, opportunityBase, buildSummary};
