import Ember from 'ember';

export function formatHashrate(params/*, hash*/) {
  var hashrate = params[0];
  var i = 0;
  var units = ['Sol', 'KSol', 'MSol', 'GSol', 'TSol', 'PSol'];
  while (hashrate > 1000) {
    hashrate = hashrate / 1000;
    i++;
  }
  return hashrate.toFixed(2) + ' ' + units[i];
}

export default Ember.Helper.helper(formatHashrate);
