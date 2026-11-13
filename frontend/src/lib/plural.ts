/**
 * plural picks the Polish noun form for a whole count: `one` for 1, `few`
 * for counts ending in 2–4 other than 12–14, and `many` for everything
 * else, including 0. For example: 1 rejs, 2 rejsy, 5 rejsów, 12 rejsów,
 * 22 rejsy.
 */
export function plural(n: number, one: string, few: string, many: string): string {
	if (n === 1) return one;
	const units = n % 10;
	const tens = n % 100;
	if (units >= 2 && units <= 4 && (tens < 12 || tens > 14)) return few;
	return many;
}
