#!/usr/bin/env python3
"""
Per-SuperNode Everlight reward calculator.

Given a single SN's stored bytes plus the chain-wide payout policy, computes
that SN's expected reward per period and per month. Per-byte payout rate is
uniform across the fleet, so the reward is just bytes * rate.

Ramp-up and EMA smoothing are ignored here for simplicity; in steady state
they have small effects.

Defaults match the placeholder inputs in
docs/design/Cascade-Everlight-Funding-Model.md (section 2).

Examples:
    ./everlight-sn-reward.py --sn-storage-gib 2048   # regular 2 TB SN
    ./everlight-sn-reward.py --sn-storage-gib 10240  # whale 10 TB SN
    ./everlight-sn-reward.py --sn-storage-gib 1024 --storage-apr 0.50
"""

import argparse


MIN_BYTES_FOR_PAYMENT_GIB = 1.0  # min_cascade_bytes_for_payment default = 1 GiB


def compute_sn_reward(sn_storage_gib, hw_rate, storage_apr, p_lume,
                      period_blocks, block_time_sec):
    """Return per-SN reward dict at the given chain-wide policy."""
    seconds_per_period = period_blocks * block_time_sec
    weeks_per_month = (365.25 / 12) / 7  # ~= 4.348

    per_byte_monthly_usd = hw_rate * (1 + storage_apr)

    eligible = sn_storage_gib >= MIN_BYTES_FOR_PAYMENT_GIB

    if not eligible:
        return {
            "eligible": False,
            "per_byte_monthly_usd": per_byte_monthly_usd,
            "seconds_per_period": seconds_per_period,
            "sn_monthly_usd": 0.0,
            "sn_period_usd": 0.0,
            "sn_period_lume": 0.0,
            "sn_annual_usd": 0.0,
        }

    sn_monthly_usd = sn_storage_gib * per_byte_monthly_usd
    sn_period_usd = sn_monthly_usd / weeks_per_month
    sn_period_lume = sn_period_usd / p_lume
    sn_annual_usd = sn_monthly_usd * 12

    sn_hw_cost_monthly = hw_rate * sn_storage_gib  # uses chain-wide HW_rate as proxy
    sn_margin_monthly = sn_monthly_usd - sn_hw_cost_monthly
    sn_margin_pct = (sn_margin_monthly / sn_hw_cost_monthly * 100) if sn_hw_cost_monthly else 0.0

    return {
        "eligible": True,
        "per_byte_monthly_usd": per_byte_monthly_usd,
        "seconds_per_period": seconds_per_period,
        "sn_monthly_usd": sn_monthly_usd,
        "sn_period_usd": sn_period_usd,
        "sn_period_lume": sn_period_lume,
        "sn_annual_usd": sn_annual_usd,
        "sn_hw_cost_monthly": sn_hw_cost_monthly,
        "sn_margin_monthly": sn_margin_monthly,
        "sn_margin_pct": sn_margin_pct,
    }


def main():
    p = argparse.ArgumentParser(
        description="Per-SN Everlight reward calculator",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    p.add_argument("--sn-storage-gib", type=float, required=True,
                   help="This SN's stored cascade bytes in GiB")
    p.add_argument("--hw-rate", type=float, default=0.04,
                   help="Hardware cost per GiB per month, USD")
    p.add_argument("--storage-apr", type=float, default=0.20,
                   help="Storage APR paid to operators, as fraction")
    p.add_argument("--p-lume", type=float, default=0.30,
                   help="LUME spot price in USD")
    p.add_argument("--period-blocks", type=int, default=100800,
                   help="payment_period_blocks")
    p.add_argument("--block-time-sec", type=float, default=6.0,
                   help="Average block time in seconds")
    args = p.parse_args()

    r = compute_sn_reward(args.sn_storage_gib, args.hw_rate, args.storage_apr,
                          args.p_lume, args.period_blocks, args.block_time_sec)

    print("Inputs:")
    print(f"  SN storage:         {args.sn_storage_gib:,.0f} GiB "
          f"({args.sn_storage_gib / 1024:.2f} TiB)")
    print(f"  HW rate:            ${args.hw_rate:.4f}/GiB/month")
    print(f"  Storage APR:        {args.storage_apr * 100:.1f}%")
    print(f"  LUME price:         ${args.p_lume:.4f}")
    print(f"  Period:             {args.period_blocks} blocks @ {args.block_time_sec}s "
          f"= {r['seconds_per_period'] / 86400:.2f} days")
    print()
    print(f"Per-byte payout:      ${r['per_byte_monthly_usd']:.4f}/GiB/month")
    print()

    if not r["eligible"]:
        print(f"SN below floor (min_cascade_bytes_for_payment = {MIN_BYTES_FOR_PAYMENT_GIB} GiB).")
        print("This SN would NOT receive any payout under default Everlight params.")
        return

    print("Per-SN reward:")
    print(f"  Per period:         ${r['sn_period_usd']:,.2f}  "
          f"({r['sn_period_lume']:,.1f} LUME)")
    print(f"  Per month:          ${r['sn_monthly_usd']:,.2f}")
    print(f"  Per year:           ${r['sn_annual_usd']:,.2f}")
    print()
    print("Reference economics (uses chain-wide HW_rate as proxy for per-byte HW cost):")
    print(f"  Hardware cost (estimate): ${r['sn_hw_cost_monthly']:,.2f}/month")
    print(f"  Operator margin:          ${r['sn_margin_monthly']:,.2f}/month "
          f"({r['sn_margin_pct']:+.1f}% over cost)")
    print()
    print("NOTE: real hardware cost drops at scale (Hetzner AX52/AX102/AX162),")
    print("so actual margin is HIGHER than this estimate for SNs > 2 TiB.")


if __name__ == "__main__":
    main()
