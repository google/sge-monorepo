# This directory contains custom Perforce triggers

## restrict_cls.py

Sets CLs to restricted based on restrict_cls.conf.
Can be used with any operation that has a CL number.
For example:

        restrict_cl_shelve shelve-commit //restricted/... "/usr/bin/python %//tools/triggers/restrict_cls.py% %//shared/tools/triggers/restrict_cls.conf% %serverhost% %change%"
        restrict_cl_change change-commit //restricted/... "/usr/bin/python %//tools/triggers/restrict_cls.py% %//shared/tools/triggers/restrict_cls.conf% %serverhost% %change%"

In order to deploy this trigger, make sure that the trigger user (configured via restrict_cls.conf)
is logged in with long-living ticket on all VMs (commit, edge and replicas).
Additionally, make sure P4PORT is set to ssl:1666 when doing this, otherwise the ticket will not match.
Finally, please make sure the VM/workspace mapping in restrict_cls.conf is valid.
