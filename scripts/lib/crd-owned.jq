.metadata.labels["app.kubernetes.io/part-of"]=="gpu-node-selector" or
(.metadata.labels["app.kubernetes.io/part-of"]==null and
 ((.metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"] // "{}" | fromjson).spec==$desired.spec))
