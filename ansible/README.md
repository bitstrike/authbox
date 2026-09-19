
export ANSIBLE_NOCOWS=1

ansible-playbook \
  -i '10.17.34.184,' \
  -u root \
  -e platform_host=auth.cloud.bitcrash.net \
  playbooks/enroll-host.yml



# popuplate the u2f mappings to the vm after regsistering pamu2fcfg -n in authbox
# read -s CLIENT_ID
# read -s CLIENT_SECRET

$ export LINUX_AUTH_TOKEN=$(curl -sk -X POST \
  https://auth.cloud.bitcrash.net:8443/oauth/token \
  -d grant_type=client_credentials \
  -d client_id=$CLIENT_ID \
  -d client_secret=$CLIENT_SECRET | jq -r .access_token)

$ echo $LINUX_AUTH_TOKEN
c0c...20394


ansible-playbook -i '10.17.34.184,' -u root \
  -e platform_host=auth.cloud.bitcrash.net \
  playbooks/sync-fido2-mappings.yml
