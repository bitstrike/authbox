#!/bin/sh

# Pulls terraform variables from pass and runs plan + apply.
# Usage: ./terraform-launch.sh [plan|apply|destroy]

ACTION="${1:-plan}"

export TF_VAR_region=$(pass show authbox/terraform/region)
export TF_VAR_hosted_zone=$(pass show authbox/terraform/hosted_zone)
export TF_VAR_domain_name=$(pass show authbox/terraform/domain_name)
export TF_VAR_iam_user_name=$(pass show authbox/terraform/iam_user_name)

cd "$(dirname "$0")"

terraform init -input=false
terraform "$ACTION" -input=false
