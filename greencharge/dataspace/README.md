# The carbon feed from the ENACT Data & Object Space

`grid-carbon-intensity.json` is the asset `grid-carbon-intensity`, transferred on 7 October
2026 with the Dataspaces module of the ENACT SDK (v1.5.0) and saved here unedited.

| Step | What we used |
|---|---|
| Consumer connector (Management URL) | `https://sovity2-api.sedimark.work/api/management` |
| Consumer DSP URL | `https://sovity2-api.sedimark.work/api/dsp` |
| Provider DSP URL | `https://enact-dataspace.iti.gr/edc-provider/api/dsp` |
| Transfer relay | `https://sovity-download.sedimark.work` |
| Contract | offered by `connector-provider`, negotiated and finalised at 07:46 UTC |
| Transfer | completed at 07:53 UTC, 73 bytes, delivered by push to the relay |

The API key is not in this repository; the SDK keeps it in the Eclipse workspace.
Screenshots of every screen are `evidence/checklist/23` to `32`.

GreenCharge reads this file through `carbon.feed.file`. Run locally from the `greencharge`
folder it is picked up from `application.yml`; in the cluster the deploy scripts copy it to
the folder mounted into both copies and point `CARBON_FEED_FILE` at it.
