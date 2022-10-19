import React from 'react';
// import AutoSizer from 'react-virtualized-auto-sizer';
import { useMeasure } from 'react-use';
import { AbsoluteTimeRange, FieldConfigSource, toUtc } from '@grafana/data';
import { PanelRenderer } from '@grafana/runtime';
import { Field, PanelChrome, Input } from '@grafana/ui';

import { SceneObjectBase } from '../core/SceneObjectBase';
import { SceneComponentProps, SceneLayoutChildState } from '../core/types';

import { SceneDragHandle } from './SceneDragHandle';

export interface VizPanelState extends SceneLayoutChildState {
  title?: string;
  pluginId: string;
  options?: object;
  fieldConfig?: FieldConfigSource;
}

export class VizPanel extends SceneObjectBase<VizPanelState> {
  static Component = ScenePanelRenderer;
  static Editor = VizPanelEditor;

  onSetTimeRange = (timeRange: AbsoluteTimeRange) => {
    const sceneTimeRange = this.getTimeRange();
    sceneTimeRange.setState({
      raw: {
        from: toUtc(timeRange.from),
        to: toUtc(timeRange.to),
      },
      from: toUtc(timeRange.from),
      to: toUtc(timeRange.to),
    });
  };
}

function ScenePanelRenderer({ model }: SceneComponentProps<VizPanel>) {
  const { title, pluginId, options, fieldConfig, ...state } = model.useState();
  const { data } = model.getData().useState();
  const layout = model.getLayout();
  const isDraggable = layout.state.isDraggable ? state.isDraggable : false;
  const dragHandle = <SceneDragHandle layoutKey={layout.state.key!} />;
  const [panel, panelMeasure] = useMeasure();
  return (
    <div style={{ width: '100%', height: '100%' }} ref={panel}>
      <PanelChrome
        title={title}
        width={panelMeasure.width}
        height={panelMeasure.height}
        leftItems={isDraggable ? [dragHandle] : undefined}
      >
        {(innerWidth, innerHeight) => (
          <>
            <PanelRenderer
              title="Raw data"
              pluginId={pluginId}
              width={innerWidth}
              height={innerHeight}
              data={data}
              options={options}
              fieldConfig={fieldConfig}
              onOptionsChange={() => {}}
              onChangeTimeRange={model.onSetTimeRange}
            />
          </>
        )}
      </PanelChrome>
    </div>
  );
}

ScenePanelRenderer.displayName = 'ScenePanelRenderer';

function VizPanelEditor({ model }: SceneComponentProps<VizPanel>) {
  const { title } = model.useState();

  return (
    <Field label="Title">
      <Input defaultValue={title} onBlur={(evt) => model.setState({ title: evt.currentTarget.value })} />
    </Field>
  );
}
