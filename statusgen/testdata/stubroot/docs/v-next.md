# v-next (fixture)

Placeholder for the deep-strategic-questions doc. It exists because the GENERATED
`docs/streams/INTAKE.md` view boilerplate points at `docs/v-next.md`, and the
link check runs over every `docs/**.md` in the root — including the generated
views. A root with intake entries and no `docs/v-next.md` therefore fails its own
lint on a path statusgen itself wrote.

Nothing here is real content; the file is fixture scaffolding, one of the few
things the stub root needs beyond its stream.
