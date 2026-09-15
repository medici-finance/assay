### Fixed

- `commsloop` now delivers every accepted, in-lane, routed message to the addressee role's mailbox (`commsqueue.DeliverToMailbox`), so `deskcomms poll` as that role sees it and `deskcomms ack <id>` clears it; other roles poll empty, a message refused at the routing boundary or quarantined by the router is never delivered, and the executor leg stays exactly as gated before (#1166).
