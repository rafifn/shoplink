import { describe, expect, it } from 'vitest'

import { formatRupiah, MAX_SEN, parseRupiah } from './money'

// Intl puts a non-breaking space (U+00A0) between "Rp" and the number.
const NBSP = ' '

describe('formatRupiah', () => {
  it.each([
    [0, 'Rp 0'],
    [100, 'Rp 1'],
    [8_500_000, 'Rp 85.000'],
    [100_075, 'Rp 1.000,75'],
    [100_050, 'Rp 1.000,50'],
    [5, 'Rp 0,05'],
    [2_450_000_000, 'Rp 24.500.000'],
    // The largest price the API accepts: exact to the last sen, no float rounding.
    [MAX_SEN, 'Rp 90.071.992.547.409,91'],
  ])('%d sen → %s', (sen, want) => {
    expect(formatRupiah(sen)).toBe(want.replace(' ', NBSP))
  })

  it.each([1.5, Number.NaN, Number.MAX_SAFE_INTEGER + 2])('refuses %s, which is not a whole number of sen', (sen) => {
    expect(() => formatRupiah(sen)).toThrow(RangeError)
  })
})

describe('parseRupiah', () => {
  it.each([
    ['85000', 8_500_000],
    ['85.000', 8_500_000],
    ['Rp 85.000', 8_500_000],
    [`Rp${NBSP}85.000`, 8_500_000],
    ['Rp85.000', 8_500_000],
    ['Rp. 85.000', 8_500_000],
    ['rp 85.000', 8_500_000],
    ['  85.000  ', 8_500_000],
    ['1.000,75', 100_075],
    ['1000,75', 100_075],
    ['1000,5', 100_050],
    ['0', 0],
    ['0,05', 5],
    ['24.500.000', 2_450_000_000],
    ['90.071.992.547.409,91', MAX_SEN],
    // The float traps: 0.29 * 100 is 28.999999999999996 and 1.15 * 100 is 114.99999999999999.
    ['0,29', 29],
    ['1,15', 115],
    ['4,35', 435],
  ])('%j → %d sen', (input, want) => {
    expect(parseRupiah(input)).toBe(want)
  })

  it.each([
    ['empty', ''],
    ['only spaces', '   '],
    ['only Rp', 'Rp'],
    ['negative', '-5.000'],
    ['plus sign', '+5.000'],
    ['three decimals', '1.000,755'],
    ['decimal point instead of comma', '1000.75'],
    ['misplaced dots', '1.00.0'],
    ['two commas', '1,2,3'],
    ['comma without sen', '1000,'],
    ['sen without rupiah', ',50'],
    ['space inside the number', '85 000'],
    ['letters', '85k'],
    ['dollar sign', '$85'],
    ['above the safe maximum', '90.071.992.547.409,92'],
    ['far above the safe maximum', '99999999999999999999'],
  ])('rejects %s (%j)', (_name, input) => {
    expect(parseRupiah(input)).toBeNull()
  })

  it('round-trips with formatRupiah', () => {
    for (const sen of [0, 1, 99, 100, 12_345, 100_075, 8_500_000, MAX_SEN]) {
      expect(parseRupiah(formatRupiah(sen))).toBe(sen)
    }
  })
})
