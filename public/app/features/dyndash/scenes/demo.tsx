import React from 'react';
import { CoreApp, DataQueryRequest, dateMath, LoadingState, PanelData, TimeRange } from '@grafana/data';
import { Button } from '@grafana/ui';
import { Observable, of, Subscription } from 'rxjs';
import { Scene, ScenePanel, VizPanel } from '../models';
import { getDatasourceSrv } from 'app/features/plugins/datasource_srv';
import { runRequest } from 'app/features/dashboard/state/runRequest';
import { map } from 'rxjs/operators';

export function getDemoScene(name: string): Observable<Scene> {
  return new Observable(observer => {
    const scene = {
      title: 'Demo scene',
      panels: getDemoPanels(),
    };

    observer.next(scene);
  });
}

function getDemoPanels(): Observable<ScenePanel[]> {
  return new Observable<ScenePanel[]>(observer => {
    const panels: ScenePanel[] = [];

    const onButtonHit = () => {
      panels.push({
        id: panels.length.toString(),
        type: 'viz',
        title: 'Demo panel ' + panels.length,
        vizId: 'bar-gauge',
        gridPos: { x: 0, y: 0, w: 12, h: 5 },
        data: of({
          state: LoadingState.Done,
          series: [],
          timeRange: {} as TimeRange,
        } as PanelData),
      });
      observer.next([...panels]);
    };

    const onQuery = () => {
      panels.push({
        id: 'nestedScene',
        type: 'scene',
        title: 'Query scene',
        gridPos: { x: 0, y: 0, w: 12, h: 10 },
        panels: getQueryPanels(),
      });
      observer.next([...panels]);
    };

    panels.push({
      id: 'A',
      type: 'viz',
      title: 'Demo panel',
      vizId: 'bar-gauge',
      gridPos: { x: 0, y: 0, w: 12, h: 2 },
      data: of({
        state: LoadingState.Done,
        series: [],
        timeRange: {} as TimeRange,
      } as PanelData),
    });

    panels.push({
      id: 'button',
      type: 'component',
      gridPos: { x: 12, y: 0, w: 12, h: 1 },
      component: () => <Button onClick={onButtonHit}>Hit me</Button>,
    });

    panels.push({
      id: 'button2',
      type: 'component',
      gridPos: { x: 12, y: 1, w: 12, h: 1 },
      component: () => <Button onClick={onQuery}>Query stuff</Button>,
    });

    observer.next(panels);
  });
}

function getQueryPanels(): Observable<ScenePanel[]> {
  return getDemoData().pipe(
    map(data => {
      return data.series.map((series, index) => {
        return {
          id: `data-${index}`,
          type: 'viz',
          title: series.name,
          gridPos: { x: 0, y: index, w: 24, h: 1 },
        } as VizPanel;
      });
    })
  );
}

function getDemoData(): Observable<PanelData> {
  return new Observable<PanelData>(observer => {
    const request: DataQueryRequest = {
      app: CoreApp.Dashboard,
      requestId: 'request',
      timezone: 'browser',
      range: {
        from: dateMath.parse('now-1h', false)!,
        to: dateMath.parse('now', false)!,
        raw: { from: 'now-1h', to: 'now' },
      },
      interval: '10s',
      intervalMs: 1000,
      targets: [
        {
          alias: '__house_locations',
          refId: 'A',
          scenarioId: 'random_walk',
          seriesCount: 5,
        } as any,
      ],
      maxDataPoints: 500,
      scopedVars: {},
      startTime: Date.now(),
    };

    const subscription = new Subscription();

    getDatasourceSrv()
      .get('gdev-testdata')
      .then(ds => {
        subscription.add(runRequest(ds, request).subscribe(observer));
      });

    return subscription;
  });
}
