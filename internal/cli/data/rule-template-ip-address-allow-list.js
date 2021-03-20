function ipAddressAllowList(user, context, callback) {
  const allowlist = ['1.2.3.4', '2.3.4.5']; // authorized IPs
  const userHasAccess = allowlist.some(function (ip) {
    return context.request.ip === ip;
  });

  if (!userHasAccess) {
    return callback(new Error('Access denied from this IP address.'));
  }

  return callback(null, user, context);
}
