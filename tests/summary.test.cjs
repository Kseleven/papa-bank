const {test} = require('node:test');
const assert = require('node:assert/strict');
const {buildSummary, summaryPeriod, shiftSummaryDate, opportunityGrowth, opportunityBase} = require('../web/summary.js');
const tx = (ts, note, amount = 10, type = 'deposit') => ({ts, note, amount, type});

test('本月同名活动合并，排除取款、上月和未来记录', () => {
  const now = new Date(2026, 9, 9, 18);
  const records = Array.from({length: 10}, (_, i) => tx(new Date(2026, 9, 1 + i % 8, 12), '早读语文'));
  records.push(tx(new Date(2026, 9, 8), ' 早读语文 ', 20, 'withdraw'),
    tx(new Date(2026, 8, 30, 23, 59), '早读语文'),
    tx(new Date(2026, 9, 10), '早读语文'), tx(new Date(2026, 9, 9, 19), '早读语文'));
  const result = buildSummary(records, 'month', now);
  assert.equal(result.count, 10);
  assert.equal(result.amount, 100);
  assert.equal(result.days, 8);
  assert.deepEqual(result.groups, [{name: '早读语文', unnamed: false, count: 10, amount: 100}]);
});

test('本周从周一零点开始，支持跨月、跨年和周日', () => {
  const now = new Date(2027, 0, 3, 18); // 周日
  assert.equal(summaryPeriod('week', now).start.getTime(), new Date(2026, 11, 28).getTime());
  const result = buildSummary([
    tx(new Date(2026, 11, 27, 23, 59), '阅读'),
    tx(new Date(2026, 11, 28), '阅读'), tx(now, '阅读')
  ], 'week', now);
  assert.equal(result.count, 2);
  assert.equal(summaryPeriod('week', new Date(2026, 9, 5, 12)).start.getTime(), new Date(2026, 9, 5).getTime());
});

test('空白备注归组、去掉首尾空格、小数金额和空状态', () => {
  const now = new Date(2026, 9, 9, 18);
  const result = buildSummary([tx(now, '', 0.1), tx(now, '  ', 0.2), tx(now, ' 阅读 ', 5), tx(now, '阅读', 5)], 'month', now);
  assert.equal(result.amount, 10.3);
  assert.deepEqual(result.groups.map(g => [g.name, g.count, g.amount]), [['阅读', 2, 10], ['未填写备注的存入', 2, 0.3]]);
  assert.equal(buildSummary([], 'month', now).count, 0);
  assert.equal(buildSummary([tx('bad-date', '阅读')], 'week', now).count, 0);
});

test('可以查看历史月份，当前月份只统计到现在', () => {
  const now = new Date(2026, 9, 9, 18);
  const result = buildSummary([
    tx(new Date(2026, 8, 1, 12), '早读语文', 10),
    tx(new Date(2026, 8, 30, 23, 59), '早读语文', 20),
    tx(new Date(2026, 9, 1, 12), '早读语文', 30),
    tx(new Date(2026, 9, 10, 12), '早读语文', 40)
  ], 'month', now, new Date(2026, 8, 15));
  assert.equal(result.count, 2);
  assert.equal(result.amount, 30);
  assert.equal(result.start.getTime(), new Date(2026, 8, 1).getTime());

  const current = buildSummary([
    tx(new Date(2026, 9, 9, 17), '早读语文', 10),
    tx(new Date(2026, 9, 9, 19), '早读语文', 20)
  ], 'month', now, new Date(2026, 9, 1));
  assert.equal(current.count, 1);
  assert.equal(current.amount, 10);
});

test('历史月份包含月末最后一毫秒，排除下月零点；支持闰年', () => {
  const now = new Date(2026, 9, 9);
  const result = buildSummary([
    tx(new Date(2024, 1, 1), '阅读'),
    tx(new Date(2024, 1, 29, 23, 59, 59, 999), '阅读'),
    tx(new Date(2024, 2, 1), '阅读')
  ], 'month', now, new Date(2024, 1, 29));
  assert.equal(result.count, 2);
  assert.equal(result.end.getTime(), new Date(2024, 1, 29, 23, 59, 59, 999).getTime());
});

test('月份导航避免31日溢出，正确跨年；历史周覆盖周一到周日', () => {
  assert.equal(shiftSummaryDate('month', new Date(2026, 2, 31), -1).getTime(), new Date(2026, 1, 1).getTime());
  assert.equal(shiftSummaryDate('month', new Date(2026, 11, 31), 1).getTime(), new Date(2027, 0, 1).getTime());
  const previousWeek = shiftSummaryDate('week', new Date(2026, 9, 9), -1);
  const result = buildSummary([
    tx(new Date(2026, 8, 28), '阅读'),
    tx(new Date(2026, 9, 4, 23, 59, 59, 999), '阅读'),
    tx(new Date(2026, 9, 5), '阅读')
  ], 'week', new Date(2026, 9, 9), previousWeek);
  assert.equal(result.count, 2);
  assert.equal(result.start.getTime(), new Date(2026, 8, 28).getTime());
  assert.equal(result.end.getTime(), new Date(2026, 9, 4, 23, 59, 59, 999).getTime());
});

test('时间控件切换历史月份后刷新总结、支持直接选择和回到当前月份', () => {
  const fs = require('node:fs');
  const vm = require('node:vm');
  const now = new Date(2026, 9, 9, 18);
  class FixedDate extends Date {
    constructor(...args) { super(...(args.length ? args : [now.getTime()])); }
  }
  const elements = new Map();
  const document = {
    addEventListener() {},
    getElementById(id) {
      if (!elements.has(id)) elements.set(id, {setAttribute() {}});
      return elements.get(id);
    }
  };
  const context = vm.createContext({Date: FixedDate, document});
  vm.runInContext(fs.readFileSync(require.resolve('../web/summary.js'), 'utf8'), context);
  const html = fs.readFileSync(require.resolve('../web/index.html'), 'utf8');
  const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
  // 只跳过页面启动时的网络加载，执行实际控件和渲染代码。
  vm.runInContext(script.slice(0, script.lastIndexOf('\nloadAccounts().catch')), context);
  vm.runInContext(`summaryState='ready'; summaryRecs=[
    {ts:new Date(2026,8,1,12),type:'deposit',note:'早读语文',amount:30},
    {ts:new Date(2026,9,1,12),type:'deposit',note:'早读语文',amount:40}
  ]; renderSummary();`, context);
  assert.equal(elements.get('summaryPicker').value, '2026-10');
  assert.equal(elements.get('summaryPickerDisplay').textContent, '2026年10月');
  assert.equal(elements.get('summaryNext').disabled, true);
  vm.runInContext('shiftSummary(-1)', context);
  assert.equal(elements.get('summaryPicker').value, '2026-09');
  assert.equal(elements.get('summaryPickerDisplay').textContent, '2026年9月');
  assert.match(elements.get('summaryContent').innerHTML, /2026年9月/);
  assert.match(elements.get('summaryContent').innerHTML, /¥30\.00/);
  assert.doesNotMatch(elements.get('summaryContent').innerHTML, /¥40\.00/);
  assert.equal(elements.get('summaryNext').disabled, false);
  vm.runInContext("selectSummaryDate('2025-12'); shiftSummary(1)", context);
  assert.equal(elements.get('summaryPicker').value, '2026-01');
  assert.match(elements.get('summaryContent').innerHTML, /2026年1月还没有存入或取出记录/);
  vm.runInContext('summaryToday()', context);
  assert.equal(elements.get('summaryPicker').value, '2026-10');
  assert.match(elements.get('summaryContent').innerHTML, /¥40\.00/);
  vm.runInContext("selectSummaryDate('2026-11'); shiftSummary(1)", context);
  assert.equal(elements.get('summaryPicker').value, '2026-10');
  vm.runInContext("setSummaryPeriod('week'); selectSummaryDate('2026-09-30')", context);
  assert.equal(elements.get('summaryPicker').type, 'date');
  assert.equal(elements.get('summaryPickerDisplay').textContent, '2026年9月30日');
  assert.match(elements.get('summaryRange').textContent, /2026年9月28日 — 2026年10月4日/);
  assert.match(elements.get('summaryContent').innerHTML, /¥40\.00/);
});

test('导航逻辑不依赖月份控件的显示格式，历史月份可向前切换', () => {
  const now = new Date(2026, 9, 9, 18);
  let anchor = new Date(2026, 9, 1);
  anchor = shiftSummaryDate('month', anchor, -1);
  assert.equal(anchor.getTime(), new Date(2026, 8, 1).getTime());
  const result = buildSummary([
    tx(new Date(2026, 8, 30, 23, 59, 59, 999), '早读语文', 80),
    tx(new Date(2026, 9, 1), '早读语文', 20)
  ], 'month', now, anchor);
  assert.equal(result.amount, 80);
  assert.equal(result.start.getTime(), new Date(2026, 8, 1).getTime());
});

test('总结同时统计存入和取出，并按备注分别归类，净变化为存入减取出', () => {
  const now = new Date(2026, 9, 9, 18);
  const result = buildSummary([
    tx(new Date(2026, 9, 1), '早读语文', 10, 'deposit'),
    tx(new Date(2026, 9, 2), '早读语文', 15, 'deposit'),
    tx(new Date(2026, 9, 3), '买书', 8, 'withdraw'),
    tx(new Date(2026, 9, 4), '买书', 2, 'withdraw'),
    tx(new Date(2026, 9, 5), '', 1, 'withdraw')
  ], 'month', now);
  assert.equal(result.depositCount, 2);
  assert.equal(result.depositAmount, 25);
  assert.equal(result.withdrawalCount, 3);
  assert.equal(result.withdrawalAmount, 11);
  assert.equal(result.netAmount, 14);
  assert.deepEqual(result.depositGroups.map(g => [g.name, g.count, g.amount]), [['早读语文', 2, 25]]);
  assert.deepEqual(result.withdrawalGroups.map(g => [g.name, g.count, g.amount]), [['买书', 2, 10], ['未填写备注的取出', 1, 1]]);
});

test('支出影响按月利率预测未来一周、一个月和一年', () => {
  const noInterest = opportunityGrowth(100, 0);
  assert.deepEqual(noInterest.map(item => [item.key, item.total, item.gain]), [['week', 100, 0], ['month', 100, 0], ['year', 100, 0]]);
  const growth = opportunityGrowth(100, 10);
  assert.equal(growth.length, 3);
  assert.equal(growth[0].total, 102.25);
  assert.equal(growth[1].total, 110);
  assert.equal(growth[2].total, 318.87);
  assert.deepEqual(growth.map(item => item.total - item.gain), [100, 100, 100]);
});

test('所选时间段的支出按各自发生时间累计到总结时点后再预测', () => {
  const asOf = new Date(2026, 9, 9, 12);
  const records = [
    tx(new Date(2026, 9, 9, 12), '买书', 100, 'withdraw'),
    tx(new Date(2026, 9, 2, 12), '玩具', 100, 'withdraw'),
    tx(new Date(2026, 8, 30, 12), '上月', 100, 'withdraw')
  ];
  const period = buildSummary(records, 'month', asOf);
  const base = opportunityBase(period.withdrawalOpportunityRecords, 10, period.end);
  assert.equal(period.withdrawalCount, 2);
  assert.ok(base > 200);
  assert.ok(base < 202.5);
  assert.equal(opportunityGrowth(base, 10)[1].total, 222.47);
});
