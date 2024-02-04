

  export function lw_charts(container_chart,chartOptions,pair,orders,update_cadles){

    container_chart.innerHTML = '';
    container_chart.style.position = 'relative';

    let intervals = ['1m', '5m', '15m', '30m','1h','4h','1d'];
    const switcherElement = createSimpleSwitcher(intervals, intervals[0], syncToInterval);

    const chart = LightweightCharts.createChart(container_chart,chartOptions);
    container_chart.appendChild(switcherElement);

    // Отображение легенды
    const toolTip = document.createElement('div');
    toolTip.className = 'three-line-legend';
    container_chart.appendChild(toolTip);
    toolTip.style.display = 'block';
    toolTip.style.left = 3 + 'px';
    toolTip.style.top = 3 + 'px';
    toolTip.innerHTML = '<div style="font-size: 24px; margin: 4px 0px; color: #20262E">' + pair.value + '</div>';
    
    var candleSeries = null;

    function syncToInterval(interval) {
      if (candleSeries) {
        chart.removeSeries(candleSeries);
        candleSeries = null;
      }

      candleSeries = chart.addCandlestickSeries();

      let candles = update_cadles(pair.value,interval);
      candleSeries.setData(candles);

      // Отображение ордеров на графике
      let markers_chart = []; 

      for (let order of orders) {

        if (order.Pair === pair.value) {
            let timeCreated = +new Date(order.TimeCreated) / 1000 ;
            let timeFinished = +new Date(order.Time) / 1000 ;

            if (order.Side=='BUY'){
              if (order.Status!='Close'){
                markers_chart.push({ time:timeCreated , position: 'belowBar', color: '#00ff00', shape: 'arrowUp', text: `long ${order.ID}`});
              } else {
                markers_chart.push({ time:timeCreated , position: 'belowBar', color: '#00ff00', shape: 'arrowUp', text: `long ${order.ID}`});
                markers_chart.push({ time: timeFinished, position: 'aboveBar', color: '#e91e63', shape: 'arrowDown', text: `long ${order.ID}`});
              }
            }
            if (order.Side=='SELL'){
              if (order.Status!='Close'){
                markers_chart.push({ time:timeCreated , position: 'belowBar', color: '#00ff00', shape: 'arrowDown', text: `short ${order.ID}`});
              } else {
                markers_chart.push({ time:timeCreated , position: 'belowBar', color: '#00ff00', shape: 'arrowDown', text: `short ${order.ID}`});
                markers_chart.push({ time: timeFinished, position: 'aboveBar', color: '#e91e63', shape: 'arrowUp', text: `short ${order.ID}`});
              }
            }    
        } 
    }

    candleSeries.setMarkers(markers_chart);


    }

    syncToInterval(intervals[0]);
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



  function createSimpleSwitcher(items, activeItem, activeItemChangedCallback) {
    var switcherElement = document.createElement('div');
    switcherElement.classList.add('switcher');
  
    var intervalElements = items.map(function(item) {
      var itemEl = document.createElement('button');
      itemEl.innerText = item;
      itemEl.classList.add('switcher-item');
      itemEl.classList.toggle('switcher-active-item', item === activeItem);
      itemEl.addEventListener('click', function() {
        onItemClicked(item);
      });
      switcherElement.appendChild(itemEl);
      return itemEl;
    });
  
    function onItemClicked(item) {
      if (item === activeItem) {
        return;
      }
  
      intervalElements.forEach(function(element, index) {
        element.classList.toggle('switcher-active-item', items[index] === item);
      });
  
      activeItem = item;
  
      activeItemChangedCallback(item);
    }
  
    return switcherElement;
  }