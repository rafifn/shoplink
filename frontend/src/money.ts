// Money is rupiah in whole sen (1/100 rupiah), the same integers the API uses:
// Rp 1.000,75 is 100075. These helpers never use floating point for money, so
// the classic traps (0.29 * 100 = 28.999999999999996) can't happen.

/** The largest price the API accepts: Number.MAX_SAFE_INTEGER sen, about Rp 90 trillion. */
export const MAX_SEN = Number.MAX_SAFE_INTEGER

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
})

/**
 * Shows sen as Indonesian rupiah: whole amounts without decimals
 * (8500000 → "Rp 85.000"), others with two (100075 → "Rp 1.000,75").
 * Intl separates "Rp" and the number with a non-breaking space.
 */
export function formatRupiah(sen: number): string {
  if (!Number.isSafeInteger(sen)) {
    throw new RangeError(`formatRupiah needs a whole number of sen, got ${sen}`)
  }
  const abs = BigInt(Math.abs(sen))
  // Intl's default for IDR would round sen away (Rp 1.000,75 → "Rp 1.001"),
  // so format the whole rupiah and add the sen ourselves.
  const whole = rupiah.format(abs / 100n)
  const cents = abs % 100n
  const text = cents === 0n ? whole : `${whole},${cents.toString().padStart(2, '0')}`
  return sen < 0 ? `-${text}` : text
}

// "85000", "85.000", "1.000,75", "1000,5": optional "Rp" or "Rp.", dots only as
// thousands separators (groups of three), and at most two digits of sen after a comma.
const RUPIAH_INPUT = /^(?:rp\.?)?\s*(\d+|\d{1,3}(?:\.\d{3})+)(?:,(\d{1,2}))?$/i

/**
 * Reads a price a seller typed into sen, or null if it isn't a valid,
 * non-negative amount within MAX_SEN. "1.000,75" → 100075; "1.5" and
 * "1.000,755" are rejected rather than guessed.
 */
export function parseRupiah(input: string): number | null {
  const match = RUPIAH_INPUT.exec(input.trim())
  if (!match) {
    return null
  }
  const [, whole, cents = ''] = match
  const sen = BigInt(whole.replaceAll('.', '')) * 100n + BigInt(cents.padEnd(2, '0'))
  if (sen > BigInt(MAX_SEN)) {
    return null
  }
  return Number(sen)
}
