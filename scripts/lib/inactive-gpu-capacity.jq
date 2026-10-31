# Kubelet can report zero or retain the previous count after plugin removal. Accept
# only inactive devices on this project's GPU worker, never usable capacity.
all(.items[];
  (.status.capacity[$r]==null and .status.allocatable[$r]==null) or
  (.metadata.name==$gpu and
   ((.status.capacity[$r] // "") | test("^(0|[1-9][0-9]*)$")) and
   .status.allocatable[$r]=="0"))
