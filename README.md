# iqair_exporter

This is a [Prometheus](https://prometheus.io) exporter to collect air quality data from iqAir AirVisual air quality monitors.

It has been tested with the [iqAir Air Visual Pro](https://www.iqair.com/us/air-quality-monitors/airvisual-series)

## Building
```bash
go mod tidy
go build .
```

## Usage

Get the API URL for your device from the AirVisual portal.

1. go to your dashboard
1. click Devices
1. click the vertical dots next to the device you want to monitor
1. click the API tab
1. copy the URL next to "Device API Link". It should be something like `https://www.airvisual.com/api/v2/node/<hex string>`

Now run the exporter:
```bash
./iqair_exporter --iqair.scrape-uri=$API_URL
```

Or with Docker:
```
TODO
```

By default, the exporter listens on port `9861` and exports metrics on `/metrics`
 
## Scrape Config
```
TODO
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[MIT](https://choosealicense.com/licenses/mit/)