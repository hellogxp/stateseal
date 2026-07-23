# Localization policy

StateSeal separates protocol identity from user-facing explanation. Commands,
event types, verdicts, rule IDs, hashes, receipt fields, and captured logs retain
their original values. Navigation, help text, state explanations, and core user
documentation are localized.

## Supported locales

| Locale | Language | Product UI | Core docs |
| --- | --- | --- | --- |
| `en` | English | Complete | Canonical |
| `zh-CN` | 简体中文 | Complete | Complete |
| `ja` | 日本語 | Complete | Complete |
| `ko` | 한국어 | Complete | Complete |
| `es` | Español | Complete | Complete |
| `pt-BR` | Português do Brasil | Complete | Complete |
| `de` | Deutsch | Complete | Complete |
| `fr` | Français | Complete | Complete |

Core docs mean the localized README, documentation index, getting started
guide, and Runs Console guide. Deeper protocol, threat-model, and research
references currently use English as the canonical source unless the localized
index explicitly links a maintained translation.

## Fallback

The Runs Console applies this order:

1. the locale explicitly selected by the user;
2. the first supported browser locale;
3. English.

Locale families map conservatively: `zh`, `zh-CN`, and `zh-SG` use `zh-CN`;
Portuguese locales use `pt-BR`; regional Spanish, German, French, Japanese, and
Korean locales use their base catalog. Unsupported locales fall back to English.

## Translation rules

- Keep StateSeal, commands, file names, protocol values, IDs, hashes, and log
  output unchanged.
- Translate the meaning of a state around its canonical protocol value; never
  rewrite the value stored in a receipt or event.
- Use short, active sentences and culturally neutral examples.
- Keep Markdown structure and links aligned with the English source.
- Machine assistance may produce a draft, but a translation must be reviewed
  for technical meaning before it is marked complete.
- A source change to README, documentation index, getting started, or Runs
  Console must update every maintained translation in the same change.

Run the localization contract locally:

```bash
sh scripts/check-i18n.sh
```

CI also compares changed canonical core pages with their locale counterparts so
translation drift is visible at review time.
