```sh
ansible-playbook -i hosts.yml playbook.yml -e "service_user=abekoh service_group=abekoh dropbox_token=XXX" --ask-become-pass
```