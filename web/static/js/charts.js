

  export function lw_charts(container_chart,chartOptions,data,pair,orders){


    container_chart.innerHTML = '';
    container_chart.style.position = 'relative';


    const chart = LightweightCharts.createChart(container_chart,chartOptions);
    const candleSeries = chart.addCandlestickSeries();
    candleSeries.setData(data);


    // Отображение легенды
    var toolTip = document.createElement('div');
    toolTip.className = 'three-line-legend';
    container_chart.appendChild(toolTip);
    toolTip.style.display = 'block';
    toolTip.style.left = 3 + 'px';
    toolTip.style.top = 3 + 'px';
    toolTip.innerHTML = '<div style="font-size: 24px; margin: 4px 0px; color: #20262E">' + pair.value + '</div>';



    // Отображение ордеров на графике
    let markers_chart = []; 

    for (let order of orders) {

      if (order.Pair === pair.value) {
          let timeCreated = +new Date(order.TimeCreated) / 1000 ;
          let timeFinished = +new Date(order.Time) / 1000 ;

          if (order.Side=='BUY'){
              markers_chart.push({ time:timeCreated , position: 'belowBar', color: '#00ff00', shape: 'arrowUp', text: 'Buy @ '});
          }
          if (order.Side=='SELL'){
              markers_chart.push({ time: timeFinished, position: 'aboveBar', color: '#e91e63', shape: 'arrowDown', text: 'Sell @ '}); 
          }    
      } 
  }

  candleSeries.setMarkers(markers_chart);

  }



  export function widget_charts(container_chart,pair){

    let chartWidth = container_chart.clientWidth;
    new TradingView.widget(
      {
          "height": "532",
          width:chartWidth,
          // "width":"925",
          "symbol": "BINANCE:" + pair,
          "interval": "15",
          "timezone": "Europe/Moscow",
          "theme": "Light",
          "style": "1",
          "locale": "ru",
          "toolbar_bg": "#f1f3f6",
          "enable_publishing": false,
          "allow_symbol_change": true,
          "container_id": "tradingview_3418f"
      }
  );
  }