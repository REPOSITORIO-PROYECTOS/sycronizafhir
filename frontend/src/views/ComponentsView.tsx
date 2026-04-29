import { Topbar } from "@/components/layout/Topbar";
import { Card, CardContent } from "@/components/ui/card";
import { ComponentRow } from "@/components/widgets/ComponentRow";
import { useSnapshot } from "@/hooks/useSnapshot";

export function ComponentsView() {
  const { data, isLoading } = useSnapshot();

  const components = data ? Object.entries(data.components) : [];

  return (
    <>
      <Topbar
        title="Componentes"
        description="Estado granular de cada subsistema en ejecución."
      />
      <div className="flex-1 overflow-auto px-8 pb-10 pt-6">
        <Card>
          <CardContent className="space-y-2 p-4">
            <div className="grid grid-cols-12 gap-3 px-4 pb-2 text-[11px] uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Componente</div>
              <div className="col-span-2">Estado</div>
              <div className="col-span-5">Mensaje</div>
              <div className="col-span-2 text-right">Actualizado</div>
            </div>
            {isLoading && !data ? (
              <p className="px-4 py-6 text-sm text-muted-foreground">
                Cargando componentes...
              </p>
            ) : null}
            {components
              .sort((a, b) => a[0].localeCompare(b[0]))
              .map(([name, state]) => (
                <ComponentRow key={name} name={name} state={state} />
              ))}
          </CardContent>
        </Card>
      </div>
    </>
  );
}
