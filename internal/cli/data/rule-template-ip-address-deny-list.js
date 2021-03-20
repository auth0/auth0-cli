function ipAddressDenylist(user, context, callback) {
  const denylist = ['1.2.3.4', '2.3.4.5']; // unauthorized IPs
  const notAuthorized = denylist.some(function (ip) {
    return context.request.ip === ip;
  });

  if (notAuthorized) {
    return callback(
      new UnauthorizedError('Access denied from this IP address.')
    );
  }

  return callback(null, user, context);
}
