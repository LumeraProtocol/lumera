#!/usr/bin/env python3
"""
Everlight aggregate pool budget calculator.

Given the chain-wide payout policy (HW_rate, storage_APR), the LUME price,
the fleet size and average storage per SN, computes the LUME and USD
amounts that need to flow through the Everlight pool per period and per year.

Defaults match the placeholder inputs in
docs/design/Cascade-Everlight-Funding-Model.md (section 2).

Example:
    ./everlight-budget.py
    ./everlight-budget.py --storage-apr 0.30 --p-lume 0.50 --n-sn 50
"""

import argparse


def compute_budget(hw_rate, storage_apr, p_lume, n_sn, b_sn_gib,
                   period_blocks, block_time_sec):
    """Return a dict with all derived budget numbers."""
    seconds_per_period = period_blocks * block_time_sec
    seconds_per_year = 365.25 * 86400
    # 30-day reference month, so the default period (432000 blocks * 6s = 30 d)
    # gives pool_period_usd == pool_monthly_usd exactly. The funding model doc
    # uses HW_rate in $/GiB/month and treats one period as one month at default.
    seconds_per_month = 30 * 86400
    periods_per_year = seconds_per_year / seconds_per_period

    per_byte_monthly_usd = hw_rate * (1 + storage_apr)
    pool_monthly_usd = n_sn * b_sn_gib * per_byte_monthly_usd
    # Per-period budget scales with the period length, independent of week/month assumptions.
    pool_period_usd = pool_monthly_usd * (seconds_per_period / seconds_per_month)
    pool_period_lume = pool_period_usd / p_lume
    pool_annual_usd = pool_monthly_usd * 12
    pool_annual_lume = pool_annual_usd / p_lume

    return {
        "seconds_per_period": seconds_per_period,
        "periods_per_year": periods_per_year,
        "per_byte_monthly_usd": per_byte_monthly_usd,
        "total_fleet_gib": n_sn * b_sn_gib,
        "pool_monthly_usd": pool_monthly_usd,
        "pool_period_usd": pool_period_usd,
        "pool_period_lume": pool_period_lume,
        "pool_annual_usd": pool_annual_usd,
        "pool_annual_lume": pool_annual_lume,
    }


def main():
    p = argparse.ArgumentParser(
        description="Everlight aggregate pool budget calculator",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    p.add_argument("--hw-rate", type=float, default=0.04,
                   help="Hardware cost per GiB per month, USD")
    p.add_argument("--storage-apr", type=float, default=0.20,
                   help="Storage APR paid to operators, as fraction")
    p.add_argument("--p-lume", type=float, default=0.30,
                   help="LUME spot price in USD")
    p.add_argument("--n-sn", type=int, default=30,
                   help="Active eligible SuperNode count")
    p.add_argument("--b-sn-gib", type=float, default=3072,
                   help="Average per-SN cascade bytes in GiB (default 3072 = 3 TiB)")
    p.add_argument("--period-blocks", type=int, default=432000,
                   help="payment_period_blocks")
    p.add_argument("--block-time-sec", type=float, default=6.0,
                   help="Average block time in seconds")
    args = p.parse_args()

    r = compute_budget(args.hw_rate, args.storage_apr, args.p_lume,
                       args.n_sn, args.b_sn_gib,
                       args.period_blocks, args.block_time_sec)

    print("Inputs:")
    print(f"  HW rate:            ${args.hw_rate:.4f}/GiB/month")
    print(f"  Storage APR:        {args.storage_apr * 100:.1f}%")
    print(f"  LUME price:         ${args.p_lume:.4f}")
    print(f"  SN count:           {args.n_sn}")
    print(f"  Avg storage per SN: {args.b_sn_gib:,.0f} GiB ({args.b_sn_gib / 1024:.2f} TiB)")
    print(f"  Period:             {args.period_blocks} blocks @ {args.block_time_sec}s "
          f"= {r['seconds_per_period'] / 86400:.2f} days")
    print()
    print("Derived:")
    print(f"  Per-byte payout:    ${r['per_byte_monthly_usd']:.4f}/GiB/month")
    print(f"  Total fleet bytes:  {r['total_fleet_gib']:,.0f} GiB "
          f"({r['total_fleet_gib'] / 1024:.2f} TiB / {r['total_fleet_gib'] / 1024 / 1024:.2f} PiB)")
    print(f"  Periods per year:   {r['periods_per_year']:.2f}")
    print()
    print("Pool budget:")
    print(f"  Per period:         ${r['pool_period_usd']:,.2f}  "
          f"({r['pool_period_lume']:,.0f} LUME)")
    print(f"  Per month:          ${r['pool_monthly_usd']:,.2f}")
    print(f"  Per year:           ${r['pool_annual_usd']:,.2f}  "
          f"({r['pool_annual_lume']:,.0f} LUME)")
    print()
    print("Per-SN average (assumes equal storage; actual payout varies by bytes held):")
    print(f"  Per period:         ${r['pool_period_usd'] / args.n_sn:,.2f}  "
          f"({r['pool_period_lume'] / args.n_sn:,.1f} LUME)")
    print(f"  Per month:          ${r['pool_monthly_usd'] / args.n_sn:,.2f}")


if __name__ == "__main__":
    main()
