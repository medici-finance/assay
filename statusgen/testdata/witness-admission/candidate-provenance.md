# Candidate provenance: derived-board/07 partial-delivery cohort

This is the provenance receipt for `candidate-inventory.json` and `candidate-cohort.json`. Brief derived-board/10 asks for it in Task 1, Task 5 and amendment A. It covers derived-board/07 only, at one dated snapshot, on both matching paths (link trailer and historical branch/body backfill). It does not cover the rest of the repository's history.

## Snapshot

| Field | Value |
|---|---|
| Endpoint | `GET https://api.github.com/repos/medici-finance/assay/pulls`, `state=all&per_page=100`, newest-created order (the order `fetchAllPulls` uses) |
| Collection window | 2026-10-26T23:59:39Z to 2026-10-26T23:59:54Z |
| Merged-before cutoff (UTC) | 2026-10-26T23:59:39Z, the collection start |
| Pages | 24; terminal short page 24 (39 records, under `per_page=100`) |
| Records | 2339 scanned, 2177 eligible (merged at or before the cutoff) |
| Requests | 40: 24 collection pages and 16 file-list pages for 14 PRs (#736 needs two pages for its 168 files) |
| Decoded collection bytes | 52,427,911 |
| UTF-8 body bytes | 10,488,296. Task 1 measured 9,095,368 at sizing; the repository grew between the two. The bytes sit in each runner's private replay (amendment A), not in this repository |
| File-list bytes | not recorded at collection; each list is bound by `files_sha256` instead |
| Limits | 100 pages, 8 MiB per decoded response, 30 file-list pages per PR, no retries, redirects refused, 60 s client timeout |
| `snapshot_sha256` | `d74af4a3ec1ef6ca091ab33ebd91938bdf3657ab20b95c323af1f2625967fb5b` |
| `eligible_sha256` | `9936eef469b2c0fd0967a06b6c2354ecee5773bba5389abf6027af7451e415e2` |

The collection called the forge the same way `ghClient.fetchAllPulls` does (same endpoint, parameters, order and decode into `ghPull`). It kept only the fields amendment A pins. It did not run through `fetchAllPulls` itself because it had to record each page's decoded hash and byte count. Verify row 11 (`TestCandidateRecollect`) rechecks the snapshot through the real `fetchAllPulls` and `httpDoer` seam.

### Refresh from the 2026-10-21 snapshot

The first snapshot, taken on 2026-10-21, had 21 pages, 2076 records and 13 members. Its exact bytes failed the withheld-content sweep, so they were never committed; amendment A records that hold. This snapshot was taken at implementation pickup, as Task 1 requires when the snapshot has changed. The member set grew by exactly one: #3945 (merged 2026-10-21T21:49:40Z, after the first snapshot was taken) mentions `derived-board/07` in its body and carries its own `Issue: #2703` trailer, so it joins as `mention-only`. The other 13 members and their dispositions are unchanged. The added pages and records are later PRs at the head of the newest-first walk.

## Committed form and sweep result

| File | SHA-256 at commit | Contents |
|---|---|---|
| `candidate-inventory.json` | `b67ae4f064d4de21227cba8e96da49beda6d771bd806af566eebfca093f682f6` | all 2339 records in page-and-position order, as identity fields plus `kept_sha256`, `files_sha256` where a file list is read, per-page request evidence, totals and the known-answer vector. No body or head ref text |
| `candidate-cohort.json` | `c39158eaf2ba5918444f1961c49008a4eb009aad07832eb9621fb01f1631a4ac` | 14 members: disposition, reason, matching paths, merge SHA, `files_sha256`, a labelled synthetic stand-in, and the verbatim file list (253 paths in total) |

- **Committed bytes.** The repository's withheld-content sweep ran over both fixtures and this receipt. Every entry was clean, and no member's file list had to be marked `held`. That includes #1006, so it is not held.
- **Manual inspection.** The implementing worker read every committed stand-in and every committed file-list path in page-and-PR order. The inventory holds no body or ref text to read. The stand-ins hold only identity fields, the members' own `Brief:`, `Authors:` and `Issue:` lines, and the matched backfill form. Every file-list path is a path in this public repository. Independent review of this receipt against the forge happens in the PR review, before merge.
- **Private replay.** Each runner keeps the exact-bytes replay outside the repository (amendment A). The sweep was run over each record's body and head ref in the private replay at this snapshot. 38 of 99 sweep entries matched, across 88 records on 16 pages, and 1 of those records is a cohort member. This receipt gives counts only. Amendment A's 2026-10-21 figures came from a different snapshot and extraction, so they are not directly comparable.

## Cohort

| PR | Disposition | Paths | Merge SHA |
|---|---|---|---|
| #735 | partial | trailer, backfill | `d004a756bbe8f0e7920376d96c61d8d50a991914` |
| #736 | partial | trailer, backfill | `bb2079bdc89feab2bf0d65b59da3e0626eefb92e` |
| #945 | partial | trailer, backfill | `6dc3808d4471052a1e55a9615ad97b19e52eb273` |
| #1010 | partial | trailer, backfill | `aec426ab3d23edce354f84abaa66b11164aa17e0` |
| #1194 | partial | trailer, backfill | `69c7fccdcea77da3dc9b6faedba498dcbc34e69a` |
| #153 | mention-only | backfill | `c59958ec9c280be4d0608f2a753b673832ac0ac3` |
| #194 | mention-only | backfill | `ffca87a52abb159d1940fd3eb55e7c76dd6b61aa` |
| #1006 | mention-only | backfill | `29d984894506d155cae010ce26c6402388fb56ce` |
| #1251 | mention-only | backfill | `92aa88273c7a3566186819c643625e78898b2dcb` |
| #1358 | mention-only | backfill | `f645d2fcb84df5ff13880ab412645c405fd8b760` |
| #1457 | mention-only | backfill | `664e9aae383bef62ce22ed4f7353b6f3793d6921` |
| #3787 | mention-only | backfill | `a9d09f6fb6d78a525de1254285aa67d739a0edbf` |
| #3794 | mention-only | backfill | `da85bf699cd6e5c361c5c1c8c00c33f7765442fc` |
| #3945 | mention-only | backfill | `aaefe56d033061fbdb2a23d022613aa1f1135a0c` |

Each member's reason in `candidate-cohort.json` refers to brief 07's actual obligations. None of the 14 is a full delivery, so brief 07 stays in progress on both paths.

**Not members.**
- #2651 is a closed, unmerged draft. Both matching paths take merged PRs only, so it is a hash-only record on page 8.
- The full-delivery positive control (#1196 for desk-tools/27, scoped against #1062) is a different brief. It is not a derived-board/07 candidate, and its evidence lives in the existing witness-admission fixture.

## Audit bounds (Task 5)

| Class | Status here |
|---|---|
| derived-board/07 trailer and backfill candidates at this snapshot | covered |
| statusgen/26 counterexample: #3759, merged as `44829891e8f74b1520b64f4180910d96e17be96a`, delivers sub-split (a) while Tasks 3/4 and Verify 2 stay owed; board review 5592762972 rejects its false implemented promotion | deferred on #2703. It shows that this bounded cohort cannot clear other briefs. It does not authorize changing statusgen/26 declarations, generated cells or the scope of these tests |
| The audit's 34 corrected-witness candidates, 87 machine-classified cases, suspected cases and untraced pre-trailer done rows | deferred on #2703, each still unproven individually |
| #2655 and other briefs | excluded: separate follow-up, not silently cleared |
| #3787 downstream chain (the legitimate reconciliation producer's regeneration, then graph-execution/28 and /29) | excluded: separate from this brief's offline acceptance. This brief does not trigger that producer or stand in for any downstream receipt |

#2703 stays open.

## Per-page checklist

The checklist covers every record on every page, in position order. A `+` marks a record committed as hash plus stand-in, with its file list in the cohort. Every other record is hash-only: identity fields plus `kept_sha256`. A record is eligible if it merged at or before the cutoff. The page counts sum to 2339.

### Page 1

`state=all&per_page=100&page=1`: 100 records (86 eligible), decoded response SHA-256 `2110ece8ebad80e966ffafafe7090aacf36acefe9c1b267e4886493e78ed7fe9`, 2264309 bytes. All hash-only.

Records in position order: #4553 #4547 #4541 #4539 #4538 #4537 #4534 #4528 #4527 #4526 #4525 #4524 #4519 #4517 #4513 #4510 #4509 #4507 #4504 #4502 #4501 #4500 #4497 #4496 #4495 #4494 #4492 #4490 #4489 #4487 #4485 #4481 #4480 #4479 #4478 #4477 #4475 #4474 #4472 #4471 #4468 #4467 #4465 #4460 #4457 #4456 #4454 #4450 #4449 #4448 #4447 #4445 #4442 #4441 #4440 #4438 #4436 #4435 #4430 #4429 #4427 #4424 #4423 #4422 #4421 #4420 #4419 #4418 #4417 #4416 #4415 #4414 #4413 #4412 #4411 #4410 #4409 #4408 #4407 #4403 #4392 #4387 #4385 #4384 #4371 #4370 #4369 #4368 #4367 #4366 #4365 #4364 #4363 #4362 #4361 #4358 #4357 #4355 #4354 #4352

### Page 2

`state=all&per_page=100&page=2`: 100 records (88 eligible), decoded response SHA-256 `93f14169f3a4797e4f4eb6e7f644e8701dd6ab47c5f745f74a88e738afaa370e`, 2287066 bytes. All hash-only.

Records in position order: #4350 #4349 #4344 #4343 #4342 #4339 #4337 #4334 #4333 #4332 #4330 #4328 #4326 #4321 #4320 #4315 #4312 #4311 #4310 #4309 #4306 #4300 #4292 #4291 #4290 #4288 #4278 #4267 #4266 #4265 #4261 #4260 #4259 #4256 #4255 #4249 #4248 #4242 #4237 #4236 #4235 #4232 #4229 #4228 #4225 #4224 #4223 #4222 #4218 #4214 #4213 #4210 #4206 #4203 #4202 #4185 #4184 #4182 #4174 #4167 #4166 #4165 #4164 #4162 #4158 #4157 #4152 #4151 #4150 #4149 #4148 #4147 #4146 #4145 #4144 #4143 #4142 #4141 #4140 #4139 #4138 #4137 #4135 #4134 #4133 #4126 #4125 #4120 #4115 #4114 #4111 #4110 #4103 #4102 #4101 #4099 #4095 #4085 #4084 #4083

### Page 3

`state=all&per_page=100&page=3`: 100 records (96 eligible), decoded response SHA-256 `5ef3982ffa34439e5536ffd41cad32c481538ca359250e0e3d430a5341f2e4d1`, 2170228 bytes. Hash plus stand-in: #3945; all others hash-only.

Records in position order: #4078 #4077 #4075 #4074 #4073 #4069 #4068 #4067 #4066 #4065 #4064 #4063 #4057 #4056 #4055 #4054 #4051 #4049 #4047 #4042 #4041 #4040 #4036 #4030 #4028 #4018 #4014 #4005 #4001 #3999 #3996 #3995 #3994 #3993 #3992 #3991 #3989 #3982 #3981 #3980 #3975 #3974 #3973 #3972 #3971 #3970 #3969 #3968 #3967 #3964 #3963 #3959 #3958 #3957 #3950 #3949 #3948 #3947 #3946 #3945+ #3943 #3941 #3938 #3936 #3932 #3931 #3923 #3922 #3921 #3917 #3916 #3915 #3914 #3913 #3912 #3909 #3906 #3905 #3902 #3900 #3899 #3894 #3893 #3891 #3890 #3888 #3886 #3885 #3877 #3875 #3874 #3873 #3872 #3869 #3868 #3867 #3866 #3860 #3858 #3856

### Page 4

`state=all&per_page=100&page=4`: 100 records (97 eligible), decoded response SHA-256 `7dba75f08fb7aaec648cf65bda12c1c47e3a394ddeabd0e8cca0959e9dc25a3f`, 2226575 bytes. Hash plus stand-in: #3794, #3787; all others hash-only.

Records in position order: #3855 #3853 #3852 #3850 #3846 #3839 #3835 #3833 #3830 #3828 #3825 #3824 #3823 #3821 #3819 #3816 #3812 #3811 #3808 #3805 #3803 #3799 #3797 #3795 #3794+ #3793 #3789 #3788 #3787+ #3785 #3784 #3783 #3782 #3781 #3779 #3777 #3776 #3775 #3773 #3772 #3769 #3768 #3767 #3763 #3760 #3759 #3751 #3749 #3747 #3746 #3745 #3744 #3741 #3738 #3732 #3729 #3728 #3727 #3717 #3716 #3715 #3706 #3705 #3703 #3701 #3696 #3695 #3693 #3688 #3687 #3685 #3684 #3683 #3675 #3674 #3673 #3672 #3671 #3670 #3668 #3657 #3656 #3653 #3652 #3651 #3650 #3649 #3644 #3643 #3642 #3634 #3626 #3625 #3623 #3622 #3618 #3617 #3611 #3610 #3609

### Page 5

`state=all&per_page=100&page=5`: 100 records (96 eligible), decoded response SHA-256 `dea67bbdf8870509b7cecbb3d44cdcf5bc970ff23e7f3ede1c90486b1ad76fe4`, 2164941 bytes. All hash-only.

Records in position order: #3608 #3599 #3598 #3596 #3595 #3594 #3593 #3592 #3586 #3584 #3583 #3582 #3581 #3580 #3579 #3578 #3576 #3574 #3573 #3572 #3571 #3570 #3553 #3552 #3551 #3550 #3543 #3541 #3540 #3538 #3534 #3525 #3524 #3523 #3522 #3516 #3515 #3514 #3513 #3512 #3511 #3509 #3508 #3505 #3504 #3503 #3502 #3501 #3500 #3488 #3475 #3474 #3473 #3470 #3468 #3466 #3465 #3464 #3463 #3462 #3459 #3458 #3457 #3456 #3452 #3450 #3449 #3444 #3443 #3440 #3439 #3436 #3435 #3434 #3433 #3431 #3430 #3429 #3428 #3425 #3418 #3417 #3416 #3415 #3414 #3409 #3408 #3407 #3405 #3404 #3403 #3402 #3401 #3400 #3392 #3391 #3389 #3385 #3384 #3381

### Page 6

`state=all&per_page=100&page=6`: 100 records (96 eligible), decoded response SHA-256 `5f3b10eec6baec0563beb3c1ee24e75f83cf8066b9363e06bc8d9f83cb9ed457`, 2291123 bytes. All hash-only.

Records in position order: #3380 #3377 #3376 #3375 #3370 #3367 #3366 #3363 #3362 #3356 #3355 #3354 #3351 #3350 #3348 #3347 #3345 #3341 #3340 #3339 #3338 #3337 #3332 #3331 #3330 #3329 #3326 #3325 #3322 #3320 #3319 #3318 #3316 #3314 #3313 #3307 #3305 #3303 #3302 #3301 #3300 #3297 #3296 #3294 #3286 #3282 #3279 #3278 #3277 #3276 #3272 #3271 #3267 #3266 #3263 #3262 #3261 #3259 #3256 #3254 #3251 #3250 #3248 #3245 #3241 #3239 #3238 #3226 #3223 #3222 #3220 #3213 #3205 #3202 #3201 #3200 #3199 #3197 #3191 #3190 #3189 #3188 #3184 #3182 #3175 #3174 #3171 #3170 #3169 #3168 #3167 #3166 #3165 #3164 #3160 #3158 #3157 #3155 #3154 #3152

### Page 7

`state=all&per_page=100&page=7`: 100 records (86 eligible), decoded response SHA-256 `7b975f08178c9851b77936f4a2ecc0685fd51c1e3c112fd0a595a70c84e3e007`, 2390667 bytes. All hash-only.

Records in position order: #3151 #3143 #3142 #3139 #3132 #3130 #3128 #3121 #3120 #3117 #3115 #3113 #3112 #3111 #3108 #3104 #3098 #3095 #3092 #3091 #3083 #3081 #3078 #3077 #3067 #3066 #3065 #3061 #3058 #3052 #3051 #3050 #3047 #3046 #3040 #3037 #3028 #3013 #3010 #3001 #2999 #2998 #2994 #2993 #2984 #2982 #2981 #2979 #2978 #2975 #2974 #2973 #2971 #2967 #2963 #2961 #2958 #2954 #2953 #2946 #2945 #2944 #2943 #2941 #2933 #2928 #2927 #2922 #2917 #2913 #2911 #2909 #2907 #2906 #2905 #2892 #2890 #2884 #2883 #2882 #2878 #2877 #2876 #2875 #2873 #2868 #2863 #2854 #2853 #2841 #2840 #2838 #2834 #2832 #2828 #2824 #2822 #2806 #2805 #2804

### Page 8

`state=all&per_page=100&page=8`: 100 records (95 eligible), decoded response SHA-256 `22e429a40fdbba0462202b6628093aa8c30027b26a61df0554de593c9c1275a9`, 2403919 bytes. All hash-only.

Records in position order: #2803 #2799 #2795 #2788 #2783 #2774 #2773 #2772 #2771 #2770 #2769 #2767 #2766 #2765 #2764 #2763 #2762 #2761 #2758 #2757 #2756 #2755 #2754 #2752 #2750 #2743 #2742 #2741 #2739 #2733 #2731 #2725 #2720 #2719 #2717 #2714 #2708 #2707 #2706 #2702 #2700 #2699 #2697 #2695 #2693 #2689 #2687 #2685 #2676 #2675 #2671 #2660 #2659 #2657 #2655 #2653 #2652 #2651 #2648 #2637 #2633 #2632 #2631 #2629 #2628 #2627 #2626 #2625 #2620 #2619 #2618 #2616 #2613 #2611 #2610 #2601 #2598 #2594 #2593 #2582 #2568 #2561 #2560 #2559 #2552 #2551 #2548 #2547 #2543 #2542 #2541 #2540 #2538 #2536 #2535 #2533 #2531 #2530 #2525 #2524

### Page 9

`state=all&per_page=100&page=9`: 100 records (99 eligible), decoded response SHA-256 `b3e7b5296f168b89ffdc1746d9fa86469e2454ae8b56c1ee6487aad58161f67f`, 2593453 bytes. All hash-only.

Records in position order: #2520 #2518 #2517 #2516 #2515 #2514 #2512 #2510 #2508 #2506 #2503 #2501 #2499 #2497 #2492 #2490 #2478 #2476 #2474 #2473 #2472 #2471 #2470 #2468 #2459 #2457 #2456 #2455 #2454 #2452 #2451 #2450 #2449 #2447 #2446 #2445 #2441 #2435 #2433 #2431 #2427 #2426 #2425 #2424 #2423 #2420 #2418 #2417 #2415 #2412 #2411 #2410 #2409 #2407 #2406 #2403 #2402 #2401 #2399 #2397 #2396 #2394 #2392 #2391 #2390 #2386 #2385 #2384 #2383 #2382 #2381 #2380 #2379 #2378 #2377 #2376 #2374 #2372 #2370 #2366 #2364 #2363 #2362 #2361 #2360 #2351 #2346 #2343 #2341 #2338 #2336 #2332 #2331 #2330 #2329 #2328 #2327 #2326 #2324 #2323

### Page 10

`state=all&per_page=100&page=10`: 100 records (94 eligible), decoded response SHA-256 `a997953208da4714d940ebc6dd1457fb0722adfd05cec677606e6855661d904e`, 2347612 bytes. All hash-only.

Records in position order: #2322 #2319 #2314 #2313 #2312 #2311 #2309 #2308 #2307 #2306 #2304 #2302 #2301 #2300 #2299 #2298 #2295 #2292 #2290 #2289 #2287 #2285 #2284 #2281 #2278 #2274 #2272 #2269 #2268 #2266 #2264 #2262 #2261 #2260 #2259 #2258 #2255 #2252 #2251 #2250 #2249 #2248 #2247 #2245 #2243 #2239 #2238 #2237 #2236 #2234 #2231 #2226 #2225 #2221 #2220 #2219 #2218 #2213 #2211 #2207 #2206 #2203 #2200 #2198 #2197 #2195 #2194 #2193 #2192 #2190 #2189 #2188 #2187 #2186 #2185 #2184 #2183 #2182 #2180 #2178 #2177 #2174 #2173 #2172 #2171 #2169 #2168 #2167 #2166 #2165 #2163 #2162 #2161 #2160 #2159 #2155 #2153 #2152 #2149 #2148

### Page 11

`state=all&per_page=100&page=11`: 100 records (100 eligible), decoded response SHA-256 `d9f6db6eb4dd13ba8b6f24f172d6e28c31fb27a84e94ea2cd85d36a526217fe8`, 2373230 bytes. All hash-only.

Records in position order: #2147 #2146 #2143 #2141 #2139 #2138 #2136 #2135 #2133 #2130 #2126 #2124 #2122 #2120 #2118 #2117 #2116 #2115 #2113 #2112 #2110 #2107 #2102 #2101 #2096 #2095 #2094 #2089 #2087 #2084 #2082 #2081 #2076 #2075 #2072 #2069 #2068 #2066 #2065 #2064 #2063 #2062 #2059 #2056 #2054 #2053 #2052 #2051 #2049 #2047 #2046 #2044 #2043 #2042 #2041 #2040 #2039 #2038 #2037 #2036 #2033 #2031 #2030 #2029 #2024 #2023 #2017 #2014 #2013 #2011 #2010 #2009 #2007 #2004 #2003 #2000 #1999 #1997 #1996 #1995 #1993 #1991 #1988 #1987 #1984 #1982 #1981 #1978 #1976 #1974 #1973 #1972 #1970 #1966 #1963 #1961 #1957 #1956 #1955 #1950

### Page 12

`state=all&per_page=100&page=12`: 100 records (79 eligible), decoded response SHA-256 `367e057425397435bd21ca153e80e130d761e84664f41ac2024e673722f220aa`, 2305768 bytes. All hash-only.

Records in position order: #1949 #1948 #1945 #1944 #1943 #1931 #1930 #1929 #1919 #1918 #1917 #1914 #1913 #1908 #1904 #1903 #1900 #1893 #1891 #1887 #1886 #1884 #1882 #1881 #1880 #1879 #1878 #1877 #1876 #1873 #1872 #1871 #1870 #1868 #1867 #1866 #1865 #1863 #1861 #1860 #1859 #1858 #1856 #1853 #1851 #1850 #1846 #1845 #1842 #1837 #1835 #1832 #1828 #1825 #1824 #1822 #1821 #1820 #1819 #1818 #1817 #1816 #1814 #1813 #1812 #1811 #1809 #1808 #1807 #1806 #1803 #1801 #1796 #1790 #1789 #1783 #1781 #1780 #1779 #1778 #1777 #1776 #1775 #1774 #1773 #1772 #1771 #1770 #1769 #1768 #1767 #1766 #1765 #1764 #1763 #1762 #1761 #1760 #1759 #1758

### Page 13

`state=all&per_page=100&page=13`: 100 records (78 eligible), decoded response SHA-256 `ed55efc5dd00fe6e2b130ee6ef8d7fc3f8039bcf057f9ce99876a076a2302b69`, 2299244 bytes. All hash-only.

Records in position order: #1757 #1755 #1754 #1753 #1749 #1748 #1747 #1746 #1745 #1744 #1743 #1742 #1741 #1739 #1738 #1737 #1736 #1734 #1730 #1729 #1727 #1726 #1724 #1723 #1720 #1719 #1718 #1717 #1716 #1715 #1714 #1713 #1712 #1711 #1710 #1709 #1708 #1707 #1706 #1705 #1704 #1701 #1700 #1698 #1696 #1695 #1692 #1691 #1689 #1688 #1687 #1686 #1685 #1684 #1683 #1682 #1681 #1680 #1679 #1678 #1674 #1672 #1671 #1670 #1669 #1668 #1666 #1664 #1663 #1662 #1661 #1659 #1658 #1656 #1655 #1650 #1649 #1647 #1643 #1641 #1640 #1638 #1637 #1635 #1634 #1632 #1629 #1626 #1619 #1617 #1615 #1614 #1613 #1612 #1611 #1610 #1609 #1607 #1603 #1602

### Page 14

`state=all&per_page=100&page=14`: 100 records (80 eligible), decoded response SHA-256 `40b8a33a56525ae45efbafdd27f374a436c23e341e43156eacd12d9635d0be7d`, 2197473 bytes. Hash plus stand-in: #1457; all others hash-only.

Records in position order: #1600 #1597 #1595 #1594 #1593 #1591 #1590 #1587 #1586 #1585 #1583 #1582 #1578 #1577 #1576 #1575 #1574 #1572 #1571 #1570 #1567 #1566 #1563 #1562 #1561 #1560 #1558 #1551 #1550 #1549 #1548 #1547 #1545 #1542 #1541 #1540 #1539 #1538 #1535 #1534 #1533 #1532 #1531 #1528 #1525 #1522 #1521 #1519 #1518 #1517 #1514 #1511 #1510 #1507 #1506 #1505 #1504 #1501 #1499 #1498 #1496 #1495 #1494 #1492 #1489 #1488 #1487 #1486 #1485 #1484 #1481 #1480 #1478 #1476 #1475 #1474 #1473 #1472 #1471 #1469 #1468 #1467 #1465 #1463 #1462 #1460 #1458 #1457+ #1456 #1455 #1453 #1452 #1451 #1450 #1449 #1448 #1445 #1443 #1439 #1438

### Page 15

`state=all&per_page=100&page=15`: 100 records (94 eligible), decoded response SHA-256 `04bd2770f0699058a39a168c1c9724bf5479183ce910d5c58874ac3559846471`, 2149001 bytes. Hash plus stand-in: #1358; all others hash-only.

Records in position order: #1437 #1434 #1432 #1430 #1429 #1427 #1426 #1422 #1421 #1417 #1416 #1414 #1413 #1410 #1409 #1406 #1405 #1401 #1400 #1399 #1397 #1396 #1391 #1389 #1388 #1385 #1381 #1379 #1377 #1376 #1374 #1372 #1370 #1368 #1367 #1366 #1364 #1362 #1361 #1360 #1359 #1358+ #1357 #1356 #1354 #1353 #1350 #1349 #1348 #1347 #1346 #1345 #1342 #1341 #1340 #1337 #1336 #1334 #1333 #1331 #1330 #1329 #1328 #1327 #1326 #1325 #1324 #1323 #1322 #1321 #1320 #1318 #1317 #1316 #1315 #1314 #1313 #1312 #1311 #1308 #1304 #1301 #1300 #1299 #1298 #1297 #1295 #1294 #1293 #1292 #1291 #1290 #1288 #1287 #1286 #1285 #1284 #1283 #1282 #1279

### Page 16

`state=all&per_page=100&page=16`: 100 records (96 eligible), decoded response SHA-256 `9819b7178b13fc584eb23a292c7bda930aaa827988028228e82890296d6fccc3`, 2295099 bytes. Hash plus stand-in: #1251, #1194; all others hash-only.

Records in position order: #1277 #1276 #1275 #1274 #1271 #1270 #1268 #1266 #1265 #1263 #1262 #1260 #1259 #1257 #1256 #1254 #1253 #1251+ #1244 #1243 #1241 #1238 #1237 #1236 #1235 #1231 #1230 #1229 #1228 #1227 #1226 #1225 #1222 #1221 #1220 #1219 #1215 #1214 #1212 #1211 #1210 #1209 #1206 #1200 #1199 #1198 #1196 #1194+ #1191 #1190 #1189 #1188 #1186 #1184 #1183 #1181 #1180 #1179 #1177 #1174 #1173 #1172 #1169 #1168 #1167 #1164 #1163 #1160 #1159 #1158 #1156 #1155 #1153 #1152 #1149 #1143 #1141 #1140 #1138 #1137 #1136 #1135 #1134 #1133 #1132 #1131 #1130 #1129 #1128 #1127 #1126 #1121 #1120 #1118 #1117 #1114 #1113 #1108 #1105 #1103

### Page 17

`state=all&per_page=100&page=17`: 100 records (94 eligible), decoded response SHA-256 `2771de1a1dafa91179a2e2662ddc78167e3ec9a20d050a1b62786ddadad9e86a`, 2279506 bytes. Hash plus stand-in: #1010, #1006; all others hash-only.

Records in position order: #1102 #1101 #1100 #1099 #1098 #1097 #1096 #1095 #1094 #1093 #1092 #1085 #1084 #1083 #1082 #1081 #1079 #1075 #1074 #1072 #1070 #1068 #1064 #1063 #1062 #1061 #1060 #1059 #1058 #1057 #1055 #1053 #1052 #1051 #1050 #1049 #1048 #1047 #1046 #1045 #1043 #1042 #1041 #1039 #1038 #1031 #1027 #1026 #1025 #1024 #1022 #1021 #1018 #1017 #1016 #1015 #1014 #1013 #1012 #1011 #1010+ #1009 #1008 #1006+ #1005 #1004 #1003 #1002 #1001 #1000 #998 #996 #995 #994 #993 #991 #990 #989 #988 #983 #982 #981 #980 #978 #976 #975 #974 #973 #972 #971 #969 #968 #967 #965 #962 #961 #960 #959 #958 #957

### Page 18

`state=all&per_page=100&page=18`: 100 records (98 eligible), decoded response SHA-256 `8c444c99f3a55c70ba76789808c83ca8600084c6cce2eb3cfc5e877ddb508b34`, 2223704 bytes. Hash plus stand-in: #945; all others hash-only.

Records in position order: #953 #952 #951 #950 #949 #948 #947 #945+ #944 #943 #941 #937 #936 #935 #933 #932 #930 #928 #926 #925 #924 #921 #920 #919 #917 #913 #911 #910 #909 #907 #904 #902 #901 #899 #898 #897 #894 #893 #891 #889 #888 #886 #883 #881 #878 #877 #876 #875 #873 #871 #869 #867 #866 #864 #860 #858 #856 #852 #849 #847 #846 #844 #843 #841 #840 #839 #837 #836 #835 #833 #832 #831 #829 #828 #827 #825 #824 #821 #820 #819 #818 #817 #814 #812 #810 #807 #805 #802 #800 #799 #797 #796 #794 #793 #791 #788 #787 #785 #784 #783

### Page 19

`state=all&per_page=100&page=19`: 100 records (98 eligible), decoded response SHA-256 `edd37533a6ca41026c0f3abfd3527b8605e4c85c1134cdb32f1df280e414bf2e`, 2179826 bytes. Hash plus stand-in: #736, #735; all others hash-only.

Records in position order: #782 #780 #779 #778 #777 #771 #770 #768 #767 #765 #764 #762 #761 #759 #758 #756 #753 #752 #750 #749 #746 #745 #744 #743 #741 #739 #737 #736+ #735+ #732 #729 #728 #726 #724 #723 #721 #720 #718 #714 #713 #712 #711 #707 #705 #704 #701 #698 #696 #694 #692 #691 #690 #689 #685 #684 #683 #681 #680 #679 #675 #674 #673 #672 #670 #669 #666 #664 #663 #659 #658 #657 #654 #653 #650 #647 #646 #645 #640 #638 #636 #634 #633 #631 #630 #629 #625 #623 #621 #620 #618 #617 #615 #614 #609 #608 #606 #604 #602 #601 #600

### Page 20

`state=all&per_page=100&page=20`: 100 records (100 eligible), decoded response SHA-256 `ecf21c8d06f7bdcb938d56a82ca4f03b8dbe495725b94eaa550cd1ef86914974`, 2076490 bytes. All hash-only.

Records in position order: #598 #597 #596 #594 #593 #592 #590 #589 #585 #583 #582 #580 #578 #577 #576 #575 #574 #573 #572 #571 #570 #569 #568 #567 #566 #565 #563 #562 #561 #556 #554 #553 #551 #550 #549 #547 #546 #545 #544 #543 #542 #541 #540 #539 #538 #537 #535 #534 #533 #532 #531 #529 #528 #526 #524 #521 #519 #518 #517 #515 #514 #513 #511 #510 #507 #506 #505 #504 #502 #500 #499 #498 #497 #496 #494 #493 #492 #489 #487 #485 #484 #483 #482 #481 #480 #479 #478 #477 #475 #474 #472 #469 #468 #466 #465 #464 #461 #460 #458 #457

### Page 21

`state=all&per_page=100&page=21`: 100 records (99 eligible), decoded response SHA-256 `fe4fa7a4c69bf45bc4be72337bcf729684e314997b088e2162a9992316499ca7`, 2077557 bytes. All hash-only.

Records in position order: #456 #455 #451 #450 #449 #448 #447 #445 #443 #441 #440 #439 #436 #434 #433 #432 #431 #430 #429 #428 #427 #426 #424 #423 #422 #421 #420 #417 #416 #413 #412 #411 #410 #409 #408 #407 #406 #405 #404 #403 #402 #401 #400 #399 #398 #397 #391 #389 #386 #385 #384 #383 #382 #381 #380 #378 #377 #375 #374 #373 #372 #371 #370 #369 #368 #367 #366 #365 #364 #363 #362 #360 #359 #358 #357 #356 #355 #354 #353 #352 #350 #348 #345 #344 #341 #338 #331 #329 #325 #324 #323 #321 #320 #318 #316 #314 #311 #310 #309 #308

### Page 22

`state=all&per_page=100&page=22`: 100 records (98 eligible), decoded response SHA-256 `d02766b1242b3f307bf65ea9fcd0a82919363767cc7f0389a777f30ed805cca7`, 2136102 bytes. Hash plus stand-in: #194; all others hash-only.

Records in position order: #306 #304 #303 #302 #301 #300 #299 #298 #297 #296 #295 #294 #293 #292 #290 #289 #288 #287 #286 #285 #284 #282 #279 #278 #277 #276 #275 #273 #272 #271 #270 #269 #268 #267 #266 #264 #262 #261 #260 #259 #257 #256 #255 #254 #253 #252 #251 #248 #247 #246 #245 #244 #243 #242 #241 #240 #239 #237 #234 #233 #232 #231 #230 #227 #226 #225 #222 #221 #220 #218 #215 #214 #212 #211 #210 #207 #206 #205 #204 #202 #201 #199 #198 #196 #195 #194+ #193 #192 #188 #184 #183 #181 #176 #174 #172 #171 #170 #169 #167 #166

### Page 23

`state=all&per_page=100&page=23`: 100 records (99 eligible), decoded response SHA-256 `3caf08f9b61cb6f45937b92b71dcc3085ae19c81189c5da893a3eb17ddac6055`, 1962744 bytes. Hash plus stand-in: #153; all others hash-only.

Records in position order: #165 #164 #163 #162 #161 #160 #159 #158 #156 #155 #154 #153+ #152 #151 #150 #149 #148 #147 #146 #145 #144 #143 #142 #141 #140 #139 #136 #134 #133 #132 #131 #130 #129 #128 #127 #126 #125 #124 #123 #122 #121 #119 #118 #117 #115 #113 #112 #111 #110 #109 #108 #107 #105 #104 #102 #100 #98 #97 #96 #94 #93 #91 #90 #89 #88 #87 #86 #85 #84 #83 #81 #80 #79 #78 #77 #76 #75 #74 #73 #72 #71 #69 #68 #67 #66 #65 #61 #60 #59 #58 #56 #54 #53 #52 #49 #48 #47 #46 #45 #44

### Page 24

`state=all&per_page=100&page=24`: 39 records (31 eligible), decoded response SHA-256 `e7e66e3a8f9820c61d1770ef9707d73eaff4e7d90c0784cf03673de85e1e7e86`, 732274 bytes. All hash-only.

Records in position order: #42 #41 #40 #39 #38 #37 #36 #35 #34 #33 #32 #31 #30 #29 #28 #27 #26 #25 #24 #23 #22 #20 #18 #16 #15 #14 #13 #12 #11 #10 #9 #8 #7 #6 #5 #4 #3 #2 #1
