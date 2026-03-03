# LLM Evaluation for Ozzie — March 2026

## Roles

| Role | Needs | Constraints |
|------|-------|-------------|
| **Planner** (main agent) | Reasoning, task decomposition, native French, tool calling | Quality > speed, long context |
| **Task** (sub-agent) | Code, tool execution, technical precision | Speed + code quality, reliable tool use |

---

## 1. Official Cloud — Price/Quality Comparison

### Pricing Table (sorted by input price)

| Model | Input $/MTok | Output $/MTok | Context | Category |
|-------|-------------|---------------|---------|----------|
| Mistral Small 3.2 | **$0.06** | $0.18 | 128K | Budget |
| Gemini 2.0 Flash | **$0.10** | $0.40 | 1M | Budget |
| DeepSeek V3.2 (chat) | **$0.28** | $0.42 | 128K | Budget |
| Gemini 2.5 Flash | **$0.30** | $2.50 | 1M | Value |
| Codestral | **$0.30** | $0.90 | 256K | Code |
| Grok 3 Mini | **$0.30** | $0.50 | 131K | Value |
| o3 | **$0.40** | $1.60 | 200K | Reasoning |
| Mistral Large 3 | **$0.50** | $1.50 | 256K | Mid-tier |
| Claude Haiku 4.5 | **$1.00** | $5.00 | 200K | Mid-tier |
| o4-mini | **$1.10** | $4.40 | 200K | Reasoning |
| Gemini 2.5 Pro | **$1.25** | $10.00 | 1M | Premium |
| GPT-4.1 | **$2.00** | $8.00 | 1M | Premium |
| GPT-4o | **$2.50** | $10.00 | 128K | Premium |
| Claude Sonnet 4.6 | **$3.00** | $15.00 | 200K-1M | Premium |
| Grok 3 / Grok 4 | **$3.00** | $15.00 | 131K | Premium |
| Claude Opus 4.6 | **$5.00** | $25.00 | 200K-1M | Flagship |

### Provider Details

#### Anthropic (Claude)

| Model | Input / 1M | Output / 1M | Context | Notes |
|-------|-----------|-------------|---------|-------|
| Claude Opus 4.6 | $5.00 | $25.00 | 200K (1M beta) | Best Anthropic model. SWE-bench ~80.8% |
| Claude Sonnet 4.6 | $3.00 | $15.00 | 200K (1M beta) | Good perf/price. Long context >200K: $6/$22.50 |
| Claude Haiku 4.5 | $1.00 | $5.00 | 200K | Fast, good tool use (346 tokens overhead) |

#### OpenAI

| Model | Input / 1M | Output / 1M | Context | Notes |
|-------|-----------|-------------|---------|-------|
| GPT-4o | $2.50 | $10.00 | 128K | Multimodal flagship. Cached input $1.25 |
| GPT-4.1 | $2.00 | $8.00 | 1M | Massive 1M context |
| o3 | $0.40 | $1.60 | 200K | Reasoning. Hidden reasoning tokens billed as output |
| o4-mini | $1.10 | $4.40 | 200K | Compact reasoning |

> **Note**: o-series models use internal reasoning tokens not visible in the response but billed at output price. A 500-token visible response can consume 2000+ tokens total.

#### Google (Gemini)

| Model | Input / 1M | Output / 1M | Context | Notes |
|-------|-----------|-------------|---------|-------|
| Gemini 2.5 Pro | $1.25 | $10.00 | 1M | >200K input: $2.50/$15.00. Best value for long-context |
| Gemini 2.5 Flash | $0.30 | $2.50 | 1M | Very economical. Thinking tokens at $1.25/M |
| Gemini 2.0 Flash | $0.10 | $0.40 | 1M | Ultra low-cost |

> **Note**: Google offers a generous free tier (RPM/RPD limited). Batch API with 50% additional discount.

#### xAI (Grok)

| Model | Input / 1M | Output / 1M | Context | Notes |
|-------|-----------|-------------|---------|-------|
| Grok 3 | $3.00 | $15.00 | 131K | Reasoning. Tool use via web/X search, code exec |
| Grok 3 Mini | $0.30 | $0.50 | 131K | Budget-friendly |
| Grok 4 | $3.00 | $15.00 | 131K | Reasoning-only |

> **Note**: xAI charges extra for server tool invocations: web search $5/1k calls, X search $5/1k calls, code execution $5/1k calls.

#### Mistral AI

| Model | Input / 1M | Output / 1M | Context | Notes |
|-------|-----------|-------------|---------|-------|
| Mistral Large 3 | $0.50 | $1.50 | 256K | MoE, open-weight. Very competitive |
| Codestral | $0.30 | $0.90 | 256K | Code specialist. FIM support |
| Mistral Small 3.2 | $0.06 | $0.18 | 128K | Ultra economical. 24B params |

> **Note**: Best absolute pricing among western providers. All models support function calling.

#### DeepSeek

| Model | Input / 1M | Output / 1M | Context | Notes |
|-------|-----------|-------------|---------|-------|
| DeepSeek V3.2 (chat) | $0.28 (cache miss) | $0.42 | 128K | Cache hit: $0.028 |
| DeepSeek V3.2 (reasoner) | $0.28 (cache miss) | $0.42 | 128K | Same model, thinking toggle |

> **Note**: Cheapest on the market. 75-90% discount on cache hits ($0.028/MTok).

### Benchmarks (Feb/March 2026)

#### SWE-bench Verified

| Model | Score |
|-------|-------|
| Claude Opus 4.5 | **80.9%** |
| Claude Opus 4.6 | **80.8%** |
| MiniMax M2.5 | 80.2% |
| GPT-5.2 | 80.0% |
| GLM-5 | 77.8% |
| Claude Sonnet 4.5 | 77.2% |
| Gemini 3 Pro | 76.2% |

### Tier Recommendations (Cloud)

**Premium** (best quality):

| Role | Model | Est. cost/day* |
|------|-------|----------------|
| Planner | Claude Sonnet 4.6 | ~$1.50 |
| Task | Claude Sonnet 4.6 | ~$2.00 |

**Value** (best price/quality):

| Role | Model | Est. cost/day |
|------|-------|----------------|
| Planner | Gemini 2.5 Flash ($0.30/$2.50) | ~$0.15 |
| Task | Mistral Large 3 ($0.50/$1.50) or Codestral ($0.30/$0.90) | ~$0.20 |

**Budget** (minimum viable):

| Role | Model | Est. cost/day |
|------|-------|----------------|
| Planner | Mistral Small 3.2 ($0.06/$0.18) | ~$0.02 |
| Task | DeepSeek V3.2 ($0.28/$0.42) | ~$0.10 |

*Estimate: ~50 planner requests/day, ~100 task requests/day, ~2K tokens/request.

---

## 2. European Cloud (GDPR)

| Provider | Country | Models | Input price/M | OpenAI-compat | Free tier |
|----------|---------|--------|---------------|---------------|-----------|
| **Mistral AI** (La Plateforme) | France | Mistral Small/Large, Codestral | $0.06 - $0.50 | Yes | Yes |
| **Scaleway** (Generative APIs) | France (Paris) | Qwen3-235B, Llama 70B, Mistral, Gemma | EUR 0.15 - 0.90 | Yes | 1M tokens |
| **OVHcloud** (AI Endpoints) | France (Gravelines) | Llama 70B, Qwen Coder 32B, DeepSeek-R1 | Pay-per-token | Yes | TBC |
| **Infomaniak** | Switzerland | Mistral Small, Llama 70B, Qwen3-VL | CHF 0.001-0.01/10K | Yes | 1M credits |
| **Hetzner** (bare metal) | Germany | Self-deploy (Ollama/vLLM) | EUR 184-889/month fixed | N/A | No |

### Scaleway Generative APIs (Paris)

| Model | Input EUR/M | Output EUR/M |
|-------|------------|-------------|
| qwen3-235b-a22b-instruct | 0.75 | 2.25 |
| mistral-small-3.2-24b | 0.15 | 0.35 |
| devstral-2-123b | 0.40 | 2.00 |
| llama-3.3-70b-instruct | 0.90 | 0.90 |
| qwen3-coder-30b-a3b | 0.20 | 0.80 |
| gemma-3-27b-it | 0.25 | 0.50 |

### Hetzner GPU Servers

| Model | GPU | VRAM | Price/month EUR |
|-------|-----|------|-----------------|
| GEX44 | NVIDIA RTX 4000 SFF Ada | 20 GB | 184 |
| GEX131 | NVIDIA RTX PRO 6000 Blackwell | 96 GB | 889 |

### Sovereign EU Scenario

| Role | Model | Provider | Est. cost/day |
|------|-------|----------|----------------|
| Planner | Mistral Large 3 | Mistral La Plateforme | ~$0.15 |
| Task | Codestral | Mistral La Plateforme | ~$0.10 |

**~$0.25/day**, 100% data in France, native GDPR.

---

## 3. Self-Hosted — AMD Strix Halo 96 GB

### Hardware Specs

- **APU**: Ryzen AI Max+ 395 — Radeon 8060S iGPU, 40 CUs RDNA 3.5 @ 2.9 GHz
- **Theoretical perf**: 59.4 TFLOPS FP16/BF16
- **Memory bandwidth**: ~212 GB/s (DDR5-8000, 256-bit)
- **Effective VRAM**: 48 GB dedicated (BIOS) + up to 72 GB with TTM overflow
- **Best backend**: ROCm (HIP) + hipBLASLt (62% of max = 36.9 TFLOPS)
- **Required kernel**: Linux 6.16.9+

### Expected Performance

| Model size | Quantization | Tokens/s (generation) |
|------------|-------------|----------------------|
| 7-8B | Q4_K_M | ~34-38 |
| 32B | Q4_K_M | ~12-15 |
| 70B (dense) | Q4_K_M | ~5 |
| MoE (3B active) | Q4_K_M | ~25-30 |

> Key insight: **MoE models are ideal** for Strix Halo. They load many weights in VRAM but only read a fraction per token, so the limited 212 GB/s bandwidth is not the bottleneck.

### Recommended Models

| Model | Role | Params | Quant | VRAM | t/s | FR | Code | Tools |
|-------|------|--------|-------|------|-----|-----|------|-------|
| **Qwen3-Coder-Next** | Code | 80B MoE (3B active) | Q4_K_M | 48 GB | 25-30 | B | **S** | **S** |
| **Qwen 2.5 Coder 32B** | Code | 32B dense | Q8_0 | 36 GB | 12-15 | B | A+ | A |
| **Mistral Small 3.2** | Planning | 24B dense | Q8_0 | 28 GB | 14-17 | **S** | B+ | **S** |
| **Qwen3.5-27B** | Planning | 27B dense | Q8_0 | 31 GB | 13-16 | A | A | A |
| Qwen3.5-35B-A3B | Planning | 35B MoE (3B active) | Q6_K | 27 GB | 25-30 | A | A | A |
| Qwen 2.5 72B | Planning | 72B dense | Q4_K_M | 48 GB | ~5 | A | A | A |
| Codestral 22B | Code | 22B dense | Q8_0 | 25 GB | 15-18 | B | A | A- |

### Dual-Model Configurations

**Compact (fits in 48 GB dedicated):**
```
Mistral Small 3.2 Q6_K (19 GB) + Qwen 2.5 Coder 32B Q6_K (25 GB) = 44 GB
```

**Maximum quality (needs ~76 GB TTM):**
```
Mistral Small 3.2 Q8_0 (28 GB) + Qwen3-Coder-Next Q4_K_M (48 GB) = 76 GB
```

**On-demand loading** via Ollama is the most practical approach for alternating models.

### Models That Do NOT Fit

| Model | Min size | Verdict |
|-------|----------|---------|
| DeepSeek V3.2 (671B) | >135 GB | Impossible |
| Mistral Large 3 (675B MoE) | >130 GB | Impossible |
| Qwen3-235B-A22B | ~110 GB | Impossible |

---

## 4. Scenarios

### Scenario A — Maximum Quality (cloud)

| Role | Model | Est. cost/day |
|------|-------|----------------|
| Planner | **Claude Sonnet 4.6** | ~$1.50 |
| Task | **Claude Sonnet 4.6** | ~$2.00 |

~$3.50/day. Best coding and reasoning, excellent French.

### Scenario B — Best Value (cloud mix)

| Role | Model | Est. cost/day |
|------|-------|----------------|
| Planner | **Gemini 2.5 Flash** ($0.30/$2.50) | ~$0.15 |
| Task | **Codestral** ($0.30/$0.90) | ~$0.20 |

~$0.35/day — 10x cheaper than A, good quality.

### Scenario C — Sovereign EU (Mistral + Scaleway)

| Role | Model | Provider | Est. cost/day |
|------|-------|----------|----------------|
| Planner | **Mistral Large 3** | Mistral La Plateforme | ~$0.15 |
| Task | **Codestral** | Mistral La Plateforme | ~$0.10 |

~$0.25/day, 100% data in France, native GDPR.

### Scenario D — Self-Hosted (Strix Halo)

| Role | Model | Quant | t/s |
|------|-------|-------|-----|
| Planner | **Mistral Small 3.2** | Q8_0 (28 GB) | 14-17 |
| Task | **Qwen3-Coder-Next** | Q4_K_M (48 GB) | 25-30 |

$0/day (electricity only). Good speed thanks to MoE.

### Scenario E — Hybrid (Recommended)

| Role | Cloud | Local fallback (Strix Halo) |
|------|-------|------------------------------|
| Planner | **Gemini 2.5 Flash** ($0.30/M) | **Mistral Small 3.2** (14-17 t/s) |
| Task | **Codestral API** ($0.30/M) | **Qwen3-Coder-Next** (25-30 t/s) |

The Eino model registry can switch between cloud and local based on connectivity and budget. This is the most flexible approach for Ozzie's architecture.

---

## 5. French Language Support Ranking

| Tier | Models | Notes |
|------|--------|-------|
| **S** | Mistral Small 3.2, Mistral Large 3 | French company, native first-class French |
| **A** | Qwen3.5-27B, Qwen 2.5 72B, Claude Sonnet/Opus | 200+ languages, strong French |
| **B+** | Gemini 2.5 Pro/Flash, GPT-4.1, Llama 3.3 70B | Good French, occasional anglicisms |
| **B** | Qwen3-Coder-Next, Codestral, DeepSeek V3.2 | Acceptable for code tasks (mostly English anyway) |

---

## Sources

- [Anthropic Pricing](https://platform.claude.com/docs/en/about-claude/pricing)
- [OpenAI Pricing](https://openai.com/api/pricing/)
- [Google Gemini Pricing](https://ai.google.dev/gemini-api/docs/pricing)
- [xAI Models & Pricing](https://docs.x.ai/developers/models)
- [Mistral Pricing](https://mistral.ai/pricing)
- [DeepSeek Pricing](https://api-docs.deepseek.com/quick_start/pricing)
- [PricePerToken](https://pricepertoken.com)
- [SWE-bench Verified](https://epoch.ai/benchmarks/swe-bench-verified)
- [BFCL V4 Leaderboard](https://gorilla.cs.berkeley.edu/leaderboard.html)
- [Scaleway Generative APIs](https://www.scaleway.com/en/generative-apis/)
- [OVHcloud AI Endpoints](https://www.ovhcloud.com/en/public-cloud/ai-endpoints/)
- [Infomaniak AI Tools](https://www.infomaniak.com/en/hosting/ai-tools)
- [Hetzner GPU Servers](https://www.hetzner.com/dedicated-rootserver/matrix-gpu/)
- [Strix Halo llama.cpp Performance](https://strixhalo.wiki/AI/llamacpp-performance)
- [Qwen3-Coder-Next GGUF](https://huggingface.co/unsloth/Qwen3-Coder-Next-GGUF)
