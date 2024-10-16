# Concurrent Batch Processor

This component is an experimental processor, forked from the [core
OpenTelemetry Collector `batchprocessor`
component](https://github.com/open-telemetry/opentelemetry-collector/blob/main/processor/batchprocessor/README.md).
The differences in this component, relative to that component are:

1. Synchronous pipeline support: this component blocks each producer
   until the request returns with success or an error status code.
2. Unlimited concurrency: this component will start as many goroutines
   as needed to send batches through the pipeline.
   
Here is an example configuration:

```
    processors:
      concurrentbatch:
        send_batch_max_size: 1500
        send_batch_size: 1000
        timeout: 1s
```

In this configuration, the component will admit up to 128MiB of
request data before stalling.
