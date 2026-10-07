# Grid data

`gridbridge/dataset/gb-regional.json` is a cached copy of real, published grid data for Great
Britain's 14 distribution regions, from 26 September to 6 October 2026, at 30-minute resolution.

- **Source:** Carbon Intensity API, National Energy System Operator (NESO), in partnership
  with Environmental Defense Fund Europe, University of Oxford Department of Computer Science
  and WWF. <https://carbonintensity.org.uk/>
- **Licence:** CC BY 4.0.
- **Retrieved:** 6 October 2026, from `/regional/intensity/{from}/{to}`.
- **What is kept, per region and half-hour:** the forecast carbon intensity (gCO2/kWh) and the
  share of generation from wind, solar and hydro, computed from the published generation mix.
  Nothing else is changed.

How it is used: each cluster node is assigned one region as its grid zone. A node's
`enact.eu/green-ratio` label is that region's wind + solar + hydro share. GreenCharge's own
carbon feed for the four districts does not come from this data: it is the file transferred
from the ENACT dataspace (`greencharge/dataspace/`). The replay still writes a district file
(`carbon.json`) from the same half-hour, which nothing reads now. The demo replays the
data faster than real time; the speed is shown on screen.

The district names (Harbor, Riverside, Uptown, OldTown) are GreenCharge's own and fictional.
