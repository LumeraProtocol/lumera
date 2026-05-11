#!/usr/bin/env python3
"""
Everlight endowment principal sizing calculator.

Sizes the permanently-staked principal so its yield (at staking_APR) covers
the annual pool outflow computed from the same inputs as everlight-budget.py.

Optionally applies a slashing risk buffer: a fraction of yield held back to
recapitalize against validator slashing events.

Defaults match the placeholder inputs in
docs/design/Cascade-Everlight-Funding-Model.md (section 2).

Examples:
    ./everlight-endowment.py
    ./everlight-endowment.py --staking-apr 0.05      # half-yield scenario
    ./everlight-endowment.py --risk-buffer-bps 500   # 5% slashing buffer
    ./everlight-endowment.py --p-lume 0.15 --p-lume-at-sizing 0.30  # USD-stable principal
"""

import argparse


def compute_endowment(hw_rate, storage_apr, p_lume, n_sn, b_sn_gib,
                      staking_apr, risk_buffer_bps,
                      period_blocks, block_time_sec,
                      p_lume_at_sizing=None):
    """Return a dict with annual outflow and principal sizing."""
    weeks_per_month = (365.25 / 12) / 7
    p_lume_sizing = p_lume_at_sizing if p_lume_at_sizing is not None else p_lume

    per_byte_monthly_usd = hw_rate * (1 + storage_apr)
    pool_monthly_usd = n_sn * b_sn_gib * per_byte_monthly_usd
    pool_annual_usd = pool_monthly_usd * 12
    pool_annual_lume = pool_annual_usd / p_lume_sizing

    principal_lume = pool_annual_lume / staking_apr
    if risk_buffer_bps:
        buffer_factor = 1 - risk_buffer_bps / 10000.0
        principal_lume_buffered = principal_lume / buffer_factor
    else:
        principal_lume_buffered = principal_lume

    principal_usd = principal_lume * p_lume_sizing
    principal_usd_buffered = principal_lume_buffered * p_lume_sizing

    # Price-shock view: what does this principal actually deliver at current price?
    yield_annual_lume = principal_lume * staking_apr
    yield_annual_usd_at_current = yield_annual_lume * p_lume
    yield_annual_usd_at_sizing = yield_annual_lume * p_lume_sizing
    coverage_ratio = yield_annual_usd_at_current / pool_annual_usd if pool_annual_usd else 0

    return {
        "per_byte_monthly_usd": per_byte_monthly_usd,
        "pool_annual_usd": pool_annual_usd,
        "pool_annual_lume": pool_annual_lume,
        "principal_lume": principal_lume,
        "principal_usd": principal_usd,
        "principal_lume_buffered": principal_lume_buffered,
        "principal_usd_buffered": principal_usd_buffered,
        "yield_annual_lume": yield_annual_lume,
        "yield_annual_usd_at_current": yield_annual_usd_at_current,
        "yield_annual_usd_at_sizing": yield_annual_usd_at_sizing,
        "coverage_ratio": coverage_ratio,
        "p_lume_sizing": p_lume_sizing,
    }


def main():
    p = argparse.ArgumentParser(
        description="Everlight endowment principal sizing",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    p.add_argument("--hw-rate", type=float, default=0.04,
                   help="Hardware cost per GiB per month, USD")
    p.add_argument("--storage-apr", type=float, default=0.20,
                   help="Storage APR paid to operators, as fraction")
    p.add_argument("--p-lume", type=float, default=0.30,
                   help="Current LUME spot price in USD")
    p.add_argument("--p-lume-at-sizing", type=float, default=None,
                   help="LUME price assumed when sizing principal "
                        "(default: same as --p-lume; set lower to size conservatively)")
    p.add_argument("--n-sn", type=int, default=30,
                   help="Active eligible SuperNode count")
    p.add_argument("--b-sn-gib", type=float, default=3072,
                   help="Average per-SN cascade bytes in GiB (default 3072 = 3 TiB)")
    p.add_argument("--staking-apr", type=float, default=0.10,
                   help="Validator staking APR (LUME yield earned by principal)")
    p.add_argument("--risk-buffer-bps", type=int, default=0,
                   help="Slashing risk buffer in basis points (e.g., 500 = 5%%)")
    p.add_argument("--period-blocks", type=int, default=100800,
                   help="payment_period_blocks")
    p.add_argument("--block-time-sec", type=float, default=6.0,
                   help="Average block time in seconds")
    args = p.parse_args()

    r = compute_endowment(args.hw_rate, args.storage_apr, args.p_lume,
                          args.n_sn, args.b_sn_gib,
                          args.staking_apr, args.risk_buffer_bps,
                          args.period_blocks, args.block_time_sec,
                          args.p_lume_at_sizing)

    print("Inputs:")
    print(f"  HW rate:            ${args.hw_rate:.4f}/GiB/month")
    print(f"  Storage APR:        {args.storage_apr * 100:.1f}% (paid TO operators)")
    print(f"  LUME price (now):   ${args.p_lume:.4f}")
    if args.p_lume_at_sizing is not None and args.p_lume_at_sizing != args.p_lume:
        print(f"  LUME at sizing:     ${args.p_lume_at_sizing:.4f}  "
              f"(used to size principal)")
    print(f"  SN count:           {args.n_sn}")
    print(f"  Avg storage per SN: {args.b_sn_gib:,.0f} GiB ({args.b_sn_gib / 1024:.2f} TiB)")
    print(f"  Staking APR:        {args.staking_apr * 100:.1f}% (earned BY principal)")
    print(f"  Risk buffer:        {args.risk_buffer_bps} bps "
          f"({args.risk_buffer_bps / 100:.2f}%)")
    print()
    print("Annual outflow target:")
    print(f"  USD:                ${r['pool_annual_usd']:,.2f}")
    print(f"  LUME:               {r['pool_annual_lume']:,.0f}")
    print()
    print("Endowment principal (no risk buffer):")
    print(f"  LUME:               {r['principal_lume']:,.0f}")
    print(f"  USD-equivalent:     ${r['principal_usd']:,.0f}")

    if args.risk_buffer_bps:
        print()
        print(f"Endowment principal (with {args.risk_buffer_bps} bps buffer):")
        print(f"  LUME:               {r['principal_lume_buffered']:,.0f}")
        print(f"  USD-equivalent:     ${r['principal_usd_buffered']:,.0f}")

    # Price-shock view if current price differs from sizing price
    if args.p_lume_at_sizing is not None and args.p_lume_at_sizing != args.p_lume:
        print()
        print("Price-shock view (LUME = ${:.4f}, sized at ${:.4f}):".format(
            args.p_lume, r["p_lume_sizing"]))
        print(f"  Annual yield:                  {r['yield_annual_lume']:,.0f} LUME")
        print(f"  Annual yield USD value:        ${r['yield_annual_usd_at_current']:,.2f}")
        print(f"  Coverage of USD target:        {r['coverage_ratio'] * 100:.1f}%")
        if r["coverage_ratio"] > 1.0:
            extra = r["yield_annual_usd_at_current"] - r["pool_annual_usd"]
            print(f"  -> over-funds target by:       ${extra:,.2f}/year")
            print(f"     (operators receive USD windfall)")
        elif r["coverage_ratio"] < 1.0:
            shortfall = r["pool_annual_usd"] - r["yield_annual_usd_at_current"]
            print(f"  -> under-funds target by:      ${shortfall:,.2f}/year")
            print(f"     (operator USD margin compresses or fleet thins)")


if __name__ == "__main__":
    main()
