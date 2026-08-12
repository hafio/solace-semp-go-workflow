#!/bin/bash

# Define the output file
OUTPUT_FILE="bulk-queue.yaml"

echo "Generating 700 queue blocks..."

echo "workflow-templates:

  - name: \"new\"
    inputs:
      required: []
      optional: {}

    actions:
" > ${OUTPUT_FILE}

for i in $(seq 1 700)
do
    # printf %03d ensures the number is 3 digits (001, 002, etc.)
    QUEUE_ID=$(printf "%03d" $i)

    cat <<EOF >> "$OUTPUT_FILE"
      - name: "Create Queue"
        module: "queue.add"
        args:
          queueName: "BULKQ-${QUEUE_ID}"
          accessType: exclusive
          owner: "default"
          permission: "delete"
          ingressEnabled: true
          egressEnabled: true
      - name: "Add Subscription"
        module: "q_sub.add"
        args:
          queueName: "BULKQ-${QUEUE_ID}"
          subscriptionTopic: "BULKQ/TEST"
EOF
done

echo "Done! 700 blocks written to $OUTPUT_FILE"