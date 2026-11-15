// 请求规则（表达式计费 formula_rules）的结构化模型 ⇄ 表达式文本互转。
//
// 表达式规范形态（与后端 formula 引擎的规则识别一致）：每条规则一个带括号的因子
// 「(条件 ? 倍数 : 1)」，多条规则以 * 连乘；条件为若干子条件以 && 相连。
// 后端只把「<探针条件> ? <数字> : 1」识别为规则并在账单留痕命中状态，
// 因此可视化编辑器只生成这一种形态，解析也只接受这一形态（其余写法保留为表达式模式）。

export type ParamOp = 'eq' | 'neq' | 'contains' | 'exists' | 'gt' | 'gte' | 'lt' | 'lte'
export type HeaderOp = 'eq' | 'neq' | 'contains' | 'exists'

export type RuleCondition =
	| { kind: 'weekday'; days: number[] } // 0=周日 … 6=周六
	| { kind: 'time'; start: string; end: string } // HH:mm，结束早于开始 = 跨午夜
	| { kind: 'param'; path: string; op: ParamOp; value: string }
	| { kind: 'header'; name: string; op: HeaderOp; value: string }

export type ConditionKind = RuleCondition['kind']

export interface RequestRule {
	multiplier: number
	timezone: string // 空 = 平台计费时区
	conditions: RuleCondition[]
}

export const WEEKDAY_LABELS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

export const PARAM_OP_LABELS: Record<ParamOp, string> = {
	eq: '等于',
	neq: '不等于',
	contains: '包含',
	exists: '存在',
	gt: '大于',
	gte: '大于等于',
	lt: '小于',
	lte: '小于等于',
}

export const HEADER_OP_LABELS: Record<HeaderOp, string> = {
	eq: '等于',
	neq: '不等于',
	contains: '包含',
	exists: '存在',
}

const NUMERIC_OPS: Record<string, string> = { gt: '>', gte: '>=', lt: '<', lte: '<=' }
const NUMBER_RE = /^-?\d+(\.\d+)?$/

export function newCondition(kind: ConditionKind): RuleCondition {
	switch (kind) {
		case 'weekday':
			return { kind, days: [1, 2, 3, 4, 5] }
		case 'time':
			return { kind, start: '09:00', end: '18:00' }
		case 'param':
			return { kind, path: '', op: 'eq', value: '' }
		case 'header':
			return { kind, name: '', op: 'eq', value: '' }
	}
}

export function cloneRule(rule: RequestRule): RequestRule {
	return JSON.parse(JSON.stringify(rule))
}

// ============================ 序列化 ============================

function tzArg(tz: string): string {
	return tz ? JSON.stringify(tz) : ''
}

function toMinutes(hhmm: string): number {
	const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm || '')
	if (!m) return NaN
	return Number(m[1]) * 60 + Number(m[2])
}

function fromMinutes(min: number): string {
	const h = Math.floor(min / 60) % 24
	const m = min % 60
	return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

function sortedDays(days: number[]): number[] {
	return [...new Set(days)].filter((d) => d >= 0 && d <= 6).sort((a, b) => a - b)
}

// 值字面量：数字/布尔按原样（JSON 请求体中的数字字段），其余按字符串
function paramLiteral(value: string): string {
	const v = value.trim()
	if (NUMBER_RE.test(v) || v === 'true' || v === 'false') return v
	return JSON.stringify(value)
}

function serializeCondition(cond: RuleCondition, tz: string): string | null {
	switch (cond.kind) {
		case 'weekday': {
			const days = sortedDays(cond.days)
			if (days.length === 0 || days.length === 7) return null
			const w = `weekday(${tzArg(tz)})`
			const contiguous = days.every((d, i) => d === days[0] + i)
			if (days.length === 1) return `${w} == ${days[0]}`
			if (contiguous) return `${w} >= ${days[0]} && ${w} <= ${days[days.length - 1]}`
			return `(${days.map((d) => `${w} == ${d}`).join(' || ')})`
		}
		case 'time': {
			const s = toMinutes(cond.start)
			let e = toMinutes(cond.end)
			if (Number.isNaN(s) || Number.isNaN(e)) return null
			if (e === 0) e = 1440 // 结束 00:00 = 当天结束
			if (s === e || (s === 0 && e === 1440)) return null
			const hourOnly = s % 60 === 0 && e % 60 === 0
			const q = hourOnly ? `hour(${tzArg(tz)})` : `(hour(${tzArg(tz)}) * 60 + minute(${tzArg(tz)}))`
			const sv = hourOnly ? s / 60 : s
			const ev = hourOnly ? e / 60 : e
			if (s < e) {
				const parts: string[] = []
				if (s > 0) parts.push(`${q} >= ${sv}`)
				if (e < 1440) parts.push(`${q} < ${ev}`)
				return parts.join(' && ')
			}
			return `(${q} >= ${sv} || ${q} < ${ev})`
		}
		case 'param': {
			const probe = `param(${JSON.stringify(cond.path.trim())})`
			switch (cond.op) {
				case 'eq':
					return `${probe} == ${paramLiteral(cond.value)}`
				case 'neq':
					return `${probe} != ${paramLiteral(cond.value)}`
				case 'contains':
					return `has(${probe}, ${JSON.stringify(cond.value)})`
				case 'exists':
					return `${probe} != nil`
				default:
					// 请求未携带该参数时 param() 为 nil，nil 与数字比较会求值报错：缺失按 0 处理
					return `(${probe} ?? 0) ${NUMERIC_OPS[cond.op]} ${cond.value.trim()}`
			}
		}
		case 'header': {
			const probe = `header(${JSON.stringify(cond.name.trim())})`
			switch (cond.op) {
				case 'eq':
					return `${probe} == ${JSON.stringify(cond.value)}`
				case 'neq':
					return `${probe} != ${JSON.stringify(cond.value)}`
				case 'contains':
					return `has(${probe}, ${JSON.stringify(cond.value)})`
				case 'exists':
					return `${probe} != ""`
			}
		}
	}
	return null
}

function formatMultiplier(m: number): string {
	return String(Number(m.toFixed(6)))
}

export function serializeRule(rule: RequestRule): string {
	const tz = (rule.timezone || '').trim()
	const conds = rule.conditions.map((c) => serializeCondition(c, tz)).filter((c): c is string => !!c)
	// 单条件时不需要 || 组的保护括号（因子外层已有括号）
	const cond = conds.length === 1 ? stripOuterParens(conds[0]) : conds.join(' && ')
	return `(${cond} ? ${formatMultiplier(rule.multiplier)} : 1)`
}

export function serializeRules(rules: RequestRule[]): string {
	return rules.map(serializeRule).join('\n* ')
}

// ============================ 校验 ============================

// validateRule 返回中文错误原因；空串 = 通过
export function validateRule(rule: RequestRule): string {
	if (!Number.isFinite(rule.multiplier) || rule.multiplier < 0) return '倍数必须是非负数'
	if (rule.multiplier === 1) return '倍数为 1 等于不调整价格'
	if (rule.conditions.length === 0) return '至少需要一个条件'
	for (const c of rule.conditions) {
		switch (c.kind) {
			case 'weekday': {
				const n = sortedDays(c.days).length
				if (n === 0) return '星期条件至少选择一天'
				if (n === 7) return '星期全选等于不限制，请删除该条件'
				break
			}
			case 'time': {
				const s = toMinutes(c.start)
				const e = toMinutes(c.end)
				if (Number.isNaN(s) || Number.isNaN(e)) return '时间段的开始和结束时间都需要填写'
				if (s === e || (s === 0 && e === 0)) return '时间段开始与结束相同，等于不限制'
				break
			}
			case 'param':
				if (!c.path.trim()) return '请填写请求参数路径'
				if (c.op !== 'exists' && c.value === '') return `请求参数 ${c.path} 需要填写比对值`
				if (NUMERIC_OPS[c.op] && !NUMBER_RE.test(c.value.trim())) return `请求参数 ${c.path} 的「${PARAM_OP_LABELS[c.op]}」需要数字比对值`
				break
			case 'header':
				if (!c.name.trim()) return '请填写请求头名称'
				if (c.op !== 'exists' && c.value === '') return `请求头 ${c.name} 需要填写比对值`
				break
		}
	}
	return ''
}

// ============================ 文字描述 ============================

// 星期描述按中文习惯周一在前、周日在后（如「周六、周日」「周一至周五」）
function describeDays(days: number[]): string {
	const d = sortedDays(days).map((x) => (x === 0 ? 7 : x)).sort((a, b) => a - b)
	const label = (x: number) => WEEKDAY_LABELS[x % 7]
	if (d.length >= 3 && d.every((x, i) => x === d[0] + i)) return `${label(d[0])}至${label(d[d.length - 1])}`
	return d.map(label).join('、')
}

export function describeCondition(c: RuleCondition): string {
	switch (c.kind) {
		case 'weekday':
			return describeDays(c.days)
		case 'time': {
			const cross = toMinutes(c.end) !== 0 && toMinutes(c.end) <= toMinutes(c.start)
			return `${c.start}–${cross ? '次日 ' : ''}${c.end === '00:00' ? '24:00' : c.end}`
		}
		case 'param':
			return c.op === 'exists' ? `参数 ${c.path} 存在` : `参数 ${c.path} ${PARAM_OP_LABELS[c.op]} ${c.value}`
		case 'header':
			return c.op === 'exists' ? `请求头 ${c.name} 存在` : `请求头 ${c.name} ${HEADER_OP_LABELS[c.op]} ${c.value}`
	}
}

export function describeRule(rule: RequestRule): string {
	const text = rule.conditions.map(describeCondition).join(' 且 ')
	const hasTime = rule.conditions.some((c) => c.kind === 'weekday' || c.kind === 'time')
	return hasTime && rule.timezone ? `${text}（${rule.timezone}）` : text
}

// ============================ 解析 ============================

// 顶层（括号外、字符串外）扫描工具
function scanTopLevel(expr: string, visit: (i: number, depth: number) => boolean | void): void {
	let depth = 0
	let quote = ''
	for (let i = 0; i < expr.length; i++) {
		const ch = expr[i]
		if (quote) {
			if (ch === '\\') i++
			else if (ch === quote) quote = ''
			continue
		}
		if (ch === '"' || ch === "'" || ch === '`') {
			quote = ch
			continue
		}
		if (ch === '(') depth++
		else if (ch === ')') depth--
		if (visit(i, depth) === true) return
	}
}

function splitTopLevel(expr: string, sep: string): string[] {
	const parts: string[] = []
	let last = 0
	scanTopLevel(expr, (i, depth) => {
		if (depth === 0 && expr.startsWith(sep, i)) {
			// '*' 分隔时排除 '**'（幂运算）；'&&'/'||' 本身两个字符
			if (sep === '*' && (expr[i + 1] === '*' || expr[i - 1] === '*')) return
			parts.push(expr.slice(last, i))
			last = i + sep.length
		}
	})
	parts.push(expr.slice(last))
	return parts.map((p) => p.trim())
}

function indexTopLevel(expr: string, ch: string, from = 0): number {
	let found = -1
	scanTopLevel(expr, (i, depth) => {
		if (i >= from && depth === 0 && expr[i] === ch) {
			found = i
			return true
		}
	})
	return found
}

// 去掉恰好包住整个表达式的外层括号（可多层）
function stripOuterParens(expr: string): string {
	let s = expr.trim()
	while (s.startsWith('(') && s.endsWith(')')) {
		let closeAt = -1
		scanTopLevel(s, (i, depth) => {
			if (s[i] === ')' && depth === 0) {
				closeAt = i
				return true
			}
		})
		if (closeAt !== s.length - 1) break
		s = s.slice(1, -1).trim()
	}
	return s
}

const TZ = String.raw`\(\s*(?:"((?:[^"\\]|\\.)*)")?\s*\)`
const STR = String.raw`"((?:[^"\\]|\\.)*)"`
const NUM = String.raw`(-?\d+(?:\.\d+)?)`
const re = (src: string) => new RegExp(`^${src}$`)

const WEEKDAY_ATOM = re(String.raw`weekday${TZ}\s*(==|>=|<=|>|<)\s*(\d+)`)
const HOUR_ATOM = re(String.raw`hour${TZ}\s*(>=|<|>|<=)\s*(\d+)`)
const MINUTE_ATOM = re(String.raw`\(?\s*hour${TZ}\s*\*\s*60\s*\+\s*minute${TZ}\s*\)?\s*(>=|<|>|<=)\s*(\d+)`)
const PARAM_CMP = re(String.raw`param\(\s*${STR}\s*\)\s*(==|!=)\s*(.+)`)
const PARAM_NUM = re(String.raw`\(?\s*param\(\s*${STR}\s*\)\s*(?:\?\?\s*0\s*\))?\s*(>=|<=|>|<)\s*${NUM}`)
const HEADER_CMP = re(String.raw`header\(\s*${STR}\s*\)\s*(==|!=)\s*${STR}`)
const HAS_ATOM = re(String.raw`has\(\s*(param|header)\(\s*${STR}\s*\)\s*,\s*${STR}\s*\)`)

const unquote = (s: string) => JSON.parse(`"${s}"`) as string
const OP_FROM_SYMBOL: Record<string, ParamOp> = { '>': 'gt', '>=': 'gte', '<': 'lt', '<=': 'lte' }

interface ParseState {
	conds: RuleCondition[]
	tz: string | null
	wLo: number | null
	wHi: number | null
	wEq: number | null
	tLo: number | null // 分钟
	tHi: number | null
}

function useTZ(st: ParseState, tz: string | undefined): boolean {
	const v = tz ?? ''
	if (st.tz === null) {
		st.tz = v
		return true
	}
	return st.tz === v
}

// 时间下界/上界（统一换算为分钟；只支持 >= 下界与 < 上界，与序列化形态一致）
function addTimeBound(st: ParseState, op: string, minutes: number): boolean {
	if (op === '>=' && st.tLo === null) st.tLo = minutes
	else if (op === '<' && st.tHi === null) st.tHi = minutes
	else return false
	return true
}

function parseAtom(atom: string, st: ParseState): boolean {
	let m: RegExpExecArray | null
	if ((m = WEEKDAY_ATOM.exec(atom))) {
		if (!useTZ(st, m[1])) return false
		const v = Number(m[3])
		switch (m[2]) {
			case '==':
				if (st.wEq !== null) return false
				st.wEq = v
				break
			case '>=':
				st.wLo = v
				break
			case '>':
				st.wLo = v + 1
				break
			case '<=':
				st.wHi = v
				break
			case '<':
				st.wHi = v - 1
				break
		}
		return true
	}
	if ((m = MINUTE_ATOM.exec(atom))) {
		if (!useTZ(st, m[1]) || !useTZ(st, m[2])) return false
		return addTimeBound(st, m[3], Number(m[4]))
	}
	if ((m = HOUR_ATOM.exec(atom))) {
		if (!useTZ(st, m[1])) return false
		return addTimeBound(st, m[2], Number(m[3]) * 60)
	}
	if ((m = HAS_ATOM.exec(atom))) {
		const key = unquote(m[2])
		const value = unquote(m[3])
		st.conds.push(m[1] === 'param' ? { kind: 'param', path: key, op: 'contains', value } : { kind: 'header', name: key, op: 'contains', value })
		return true
	}
	if ((m = HEADER_CMP.exec(atom))) {
		const name = unquote(m[1])
		const value = unquote(m[3])
		if (m[2] === '!=' && value === '') st.conds.push({ kind: 'header', name, op: 'exists', value: '' })
		else st.conds.push({ kind: 'header', name, op: m[2] === '==' ? 'eq' : 'neq', value })
		return true
	}
	if ((m = PARAM_NUM.exec(atom))) {
		st.conds.push({ kind: 'param', path: unquote(m[1]), op: OP_FROM_SYMBOL[m[2]], value: m[3] })
		return true
	}
	if ((m = PARAM_CMP.exec(atom))) {
		const path = unquote(m[1])
		const raw = m[3].trim()
		if (raw === 'nil') {
			if (m[2] !== '!=') return false
			st.conds.push({ kind: 'param', path, op: 'exists', value: '' })
			return true
		}
		let value: string
		if (raw.startsWith('"') && raw.endsWith('"')) value = unquote(raw.slice(1, -1))
		else if (NUMBER_RE.test(raw) || raw === 'true' || raw === 'false') value = raw
		else return false
		st.conds.push({ kind: 'param', path, op: m[2] === '==' ? 'eq' : 'neq', value })
		return true
	}
	return false
}

// 括号内的 || 组：星期枚举（weekday == a || weekday == b）或跨午夜时间段（>= s || < e）
function parseOrGroup(parts: string[], st: ParseState): boolean {
	const days: number[] = []
	if (parts.every((p) => {
		const m = WEEKDAY_ATOM.exec(p)
		if (!m || m[2] !== '==' || !useTZ(st, m[1])) return false
		days.push(Number(m[3]))
		return true
	})) {
		st.conds.push({ kind: 'weekday', days: sortedDays(days) })
		return true
	}
	if (parts.length !== 2) return false
	const bounds: Record<string, number> = {}
	for (const p of parts) {
		let m = MINUTE_ATOM.exec(p)
		if (m) {
			if (!useTZ(st, m[1]) || !useTZ(st, m[2])) return false
			bounds[m[3]] = Number(m[4])
			continue
		}
		m = HOUR_ATOM.exec(p)
		if (!m || !useTZ(st, m[1])) return false
		bounds[m[2]] = Number(m[3]) * 60
	}
	if (bounds['>='] === undefined || bounds['<'] === undefined) return false
	st.conds.push({ kind: 'time', start: fromMinutes(bounds['>=']), end: fromMinutes(bounds['<']) })
	return true
}

function parseCondition(cond: string): { conds: RuleCondition[]; tz: string } | null {
	const st: ParseState = { conds: [], tz: null, wLo: null, wHi: null, wEq: null, tLo: null, tHi: null }
	for (const part of splitTopLevel(stripOuterParens(cond), '&&')) {
		const inner = stripOuterParens(part)
		const orParts = splitTopLevel(inner, '||')
		const ok = orParts.length > 1 ? parseOrGroup(orParts.map(stripOuterParens), st) : parseAtom(inner, st)
		if (!ok) return null
	}
	// 合并星期上下界 / 单值
	if (st.wEq !== null) {
		if (st.wLo !== null || st.wHi !== null) return null
		st.conds.unshift({ kind: 'weekday', days: [st.wEq] })
	} else if (st.wLo !== null || st.wHi !== null) {
		const lo = Math.max(0, st.wLo ?? 0)
		const hi = Math.min(6, st.wHi ?? 6)
		if (lo > hi) return null
		st.conds.unshift({ kind: 'weekday', days: Array.from({ length: hi - lo + 1 }, (_, i) => lo + i) })
	}
	// 合并时间上下界（下界缺省 00:00，上界缺省 24:00）
	if (st.tLo !== null || st.tHi !== null) {
		const lo = st.tLo ?? 0
		const hi = st.tHi ?? 1440
		if (lo >= hi) return null
		const at = st.conds.findIndex((c) => c.kind !== 'weekday')
		const timeCond: RuleCondition = { kind: 'time', start: fromMinutes(lo), end: hi === 1440 ? '00:00' : fromMinutes(hi) }
		if (at < 0) st.conds.push(timeCond)
		else st.conds.splice(at, 0, timeCond)
	}
	if (st.conds.length === 0) return null
	return { conds: st.conds, tz: st.tz ?? '' }
}

export type ParseResult = { ok: true; rules: RequestRule[] } | { ok: false; error: string }

// parseRules 表达式 → 规则列表；任一因子不是可视化可表达的形态即整体失败（保留表达式模式）
export function parseRules(text: string): ParseResult {
	const src = text.trim()
	if (!src) return { ok: true, rules: [] }
	const rules: RequestRule[] = []
	const factors = splitTopLevel(src, '*')
	for (let i = 0; i < factors.length; i++) {
		const f = stripOuterParens(factors[i])
		const q = indexTopLevel(f, '?')
		const c = q < 0 ? -1 : indexTopLevel(f, ':', q + 1)
		if (q < 0 || c < 0) return { ok: false, error: `第 ${i + 1} 条规则不是「条件 ? 倍数 : 1」形态` }
		const multiplier = f.slice(q + 1, c).trim()
		const fallback = f.slice(c + 1).trim()
		if (!NUMBER_RE.test(multiplier) || fallback !== '1') {
			return { ok: false, error: `第 ${i + 1} 条规则的倍数必须是数字且未命中分支为 1` }
		}
		const parsed = parseCondition(f.slice(0, q))
		if (!parsed) return { ok: false, error: `第 ${i + 1} 条规则的条件写法无法用可视化编辑器表示` }
		rules.push({ multiplier: Number(multiplier), timezone: parsed.tz, conditions: parsed.conds })
	}
	return { ok: true, rules }
}

// ============================ 模板 ============================

export const RULE_TEMPLATES: Array<{ label: string; rule: RequestRule }> = [
	{
		label: '工作时间加价',
		rule: { multiplier: 2, timezone: '', conditions: [{ kind: 'weekday', days: [1, 2, 3, 4, 5] }, { kind: 'time', start: '09:00', end: '18:00' }] },
	},
	{ label: '夜间折扣', rule: { multiplier: 0.8, timezone: '', conditions: [{ kind: 'time', start: '23:00', end: '08:00' }] } },
	{ label: '周末折扣', rule: { multiplier: 0.9, timezone: '', conditions: [{ kind: 'weekday', days: [0, 6] }] } },
	{ label: '服务档位加价', rule: { multiplier: 2, timezone: '', conditions: [{ kind: 'param', path: 'service_tier', op: 'eq', value: 'priority' }] } },
	{ label: 'beta 头加价', rule: { multiplier: 6, timezone: '', conditions: [{ kind: 'header', name: 'anthropic-beta', op: 'contains', value: 'fast-mode' }] } },
	{ label: '长视频加价', rule: { multiplier: 1.5, timezone: '', conditions: [{ kind: 'param', path: 'duration', op: 'gte', value: '10' }] } },
	{ label: '分辨率加价', rule: { multiplier: 2, timezone: '', conditions: [{ kind: 'param', path: 'resolution', op: 'eq', value: '1080p' }] } },
]
