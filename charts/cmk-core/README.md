# CMK Helm Chart
KMS2.0 helm charts

## How To

Download all dependencies as defined in the parent chart and sub charts.

```bash
helm dependency build ./charts
```

Update the dependencies after making changes to the chart.

```bash
helm dependency update ./charts
```

After changing dependencies in a sub chart, update the sub charts dependencies first
and then update the parent chart dependencies.

```bash
helm dependency update ./charts/<subchart>
helm dependency update ./charts
```

Render chart templates locally and display the output.
```bash
helm template kms2x ./charts
```

Install entire kms2x charts
```bash
helm install kms2x ./charts 
```

UnInstall kms2x entire deployment
```bash
helm uninstall kms2x
```